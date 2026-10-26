package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helios/trackline/apps/edge-gateway/internal/auth"
	"github.com/helios/trackline/apps/edge-gateway/internal/ratelimit"
)

type fakeValidator struct{ claims *auth.Claims }

func (f fakeValidator) Validate(_ context.Context, token string) (*auth.Claims, error) {
	if token != "good-jwt" {
		return nil, errors.New("bad token")
	}
	return f.claims, nil
}

type fakeKeys struct{ err error }

func (f fakeKeys) Verify(_ context.Context, key string) (*auth.Claims, error) {
	if f.err != nil {
		return nil, f.err
	}
	if key != "tl_dev_aaaa.secret" {
		return nil, auth.ErrInvalidAPIKey
	}
	c := &auth.Claims{TenantID: "tenant-from-key", Tier: "pro", Roles: []string{"operator"}}
	c.Subject = "apikey:tl_dev_aaaa"
	return c, nil
}

type fakeLimiter struct{ d ratelimit.Decision }

func (f fakeLimiter) Allow(context.Context, string, string) ratelimit.Decision { return f.d }

func newChain(keys keyVerifier, lim limiter) (http.Handler, *http.Request) {
	var seen *http.Request
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.WriteHeader(http.StatusNoContent)
	})
	v := fakeValidator{claims: &auth.Claims{TenantID: "tenant-from-jwt", Tier: "free", Roles: []string{"viewer"}}}
	h := withMiddleware(next, v, keys, lim, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		if seen != nil {
			*r = *seen
		}
	}), nil
}

var allowAll = fakeLimiter{ratelimit.Decision{Allowed: true, Limit: 60, Remaining: 59, Reset: time.Now().Add(time.Minute)}}

func do(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestForgedTrustHeadersNeverReachFleetAPI(t *testing.T) {
	h, _ := newChain(fakeKeys{}, allowAll)
	req := httptest.NewRequest("GET", "/v1/fleets", nil)
	req.Header.Set("Authorization", "Bearer good-jwt")
	req.Header.Set("X-Trackline-Tenant", "someone-else")
	req.Header.Set("X-Trackline-Roles", "owner")
	rec := do(t, h, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := req.Header.Get("X-Trackline-Tenant"); got != "tenant-from-jwt" {
		t.Fatalf("tenant header was %q", got)
	}
	if got := req.Header.Get("X-Trackline-Roles"); got != "viewer" {
		t.Fatalf("roles header was %q", got)
	}
}

func TestAPIKeyAuthenticatesAndIsNotForwarded(t *testing.T) {
	h, _ := newChain(fakeKeys{}, allowAll)
	req := httptest.NewRequest("GET", "/v1/fleets", nil)
	req.Header.Set("X-Trackline-Key", "tl_dev_aaaa.secret")
	rec := do(t, h, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := req.Header.Get("X-Trackline-Tenant"); got != "tenant-from-key" {
		t.Fatalf("tenant header was %q", got)
	}
	if req.Header.Get("X-Trackline-Key") != "" {
		t.Fatal("the api key must not be forwarded upstream")
	}
}

func TestBadAPIKeyIsUnauthorized(t *testing.T) {
	h, _ := newChain(fakeKeys{}, allowAll)
	req := httptest.NewRequest("GET", "/v1/fleets", nil)
	req.Header.Set("X-Trackline-Key", "tl_dev_aaaa.wrong")
	rec := do(t, h, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type %q", ct)
	}
}

func TestVerifierOutageIsNotAnAuthSuccess(t *testing.T) {
	h, _ := newChain(fakeKeys{err: auth.ErrVerifierDown}, allowAll)
	req := httptest.NewRequest("GET", "/v1/fleets", nil)
	req.Header.Set("X-Trackline-Key", "tl_dev_aaaa.secret")
	if rec := do(t, h, req); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMissingCredentialIsProblemJSON(t *testing.T) {
	h, _ := newChain(fakeKeys{}, allowAll)
	rec := do(t, h, httptest.NewRequest("GET", "/v1/fleets", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != float64(401) {
		t.Fatalf("not a problem document: %s", rec.Body.String())
	}
}

func TestRateLimitedResponseShape(t *testing.T) {
	lim := fakeLimiter{ratelimit.Decision{Allowed: false, Limit: 600, Remaining: 0, RetryAfter: 12 * time.Second, Reset: time.Unix(1789000912, 0)}}
	h, _ := newChain(fakeKeys{}, lim)
	req := httptest.NewRequest("GET", "/v1/fleets", nil)
	req.Header.Set("Authorization", "Bearer good-jwt")
	rec := do(t, h, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", rec.Code)
	}
	for k, want := range map[string]string{
		"Retry-After":           "12",
		"X-RateLimit-Limit":     "600",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "1789000912",
		"Content-Type":          "application/problem+json",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !strings.Contains(rec.Body.String(), "rate-limited") {
		t.Errorf("body %s", rec.Body.String())
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	h, _ := newChain(fakeKeys{}, allowAll)
	req := httptest.NewRequest("POST", "/v1/fleets", strings.NewReader(strings.Repeat("x", maxBodyBytes+1)))
	req.Header.Set("Authorization", "Bearer good-jwt")
	if rec := do(t, h, req); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestStreamTokenOnlyAcceptedOnTheStreamPath(t *testing.T) {
	h, _ := newChain(fakeKeys{}, allowAll)

	req := httptest.NewRequest("GET", "/v1/positions/stream?token=good-jwt&x=1", nil)
	if rec := do(t, h, req); rec.Code != http.StatusNoContent {
		t.Fatalf("stream path: status %d", rec.Code)
	}
	if strings.Contains(req.URL.RawQuery, "token") || req.URL.Query().Get("x") != "1" {
		t.Fatalf("token must be stripped and other params kept, got %q", req.URL.RawQuery)
	}

	other := httptest.NewRequest("GET", "/v1/fleets?token=good-jwt", nil)
	if rec := do(t, h, other); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a query token must not authenticate other paths, got %d", rec.Code)
	}
}
