package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/helios/trackline/apps/edge-gateway/internal/auth"
	"github.com/helios/trackline/apps/edge-gateway/internal/ratelimit"
)

// maxBodyBytes is the payload cap from limits.md §3.
const maxBodyBytes = 1 << 20

// The live-positions WebSocket. Browsers cannot set an Authorization header on a
// WebSocket, so this one path may carry the access token as ?token=.
const positionsStreamPath = "/v1/positions/stream"

const errorTypeBase = "https://trackline.local/errors/"

type tokenValidator interface {
	Validate(ctx context.Context, token string) (*auth.Claims, error)
}

type keyVerifier interface {
	Verify(ctx context.Context, key string) (*auth.Claims, error)
}

type limiter interface {
	Allow(ctx context.Context, tenantID, tier string) ratelimit.Decision
}

// writeProblem writes an RFC 9457 problem document (limits.md §5).
func writeProblem(w http.ResponseWriter, r *http.Request, status int, slug, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	body := map[string]any{
		"type":   errorTypeBase + slug,
		"title":  title,
		"status": status,
	}
	if detail != "" {
		body["detail"] = detail
	}
	if id := r.Header.Get(auth.HeaderRequestID); id != "" {
		body["traceId"] = id
	}
	_ = json.NewEncoder(w).Encode(body)
}

// withMiddleware wires the chain in the order auth.md requires: strip inbound
// trust headers, authenticate, rate-limit, then assert trusted context onto the
// proxied request. Reordering this chain is a security bug.
func withMiddleware(next http.Handler, validator tokenValidator, keys keyVerifier, lim limiter, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The API key travels in an X-Trackline-* header, and step 1 deletes every
		// such header, so the credential is read first. It is only ever used to
		// authenticate; it is never forwarded.
		apiKey := r.Header.Get("X-Trackline-Key")

		// Step 1 — never trust headers the caller set themselves.
		auth.StripInboundTrust(r)

		// Payload cap, before any body is read (limits.md §3).
		if r.ContentLength > maxBodyBytes {
			writeProblem(w, r, http.StatusRequestEntityTooLarge, "payload-too-large", "Payload too large",
				"Request bodies are limited to 1 MB.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		// Step 2 — authenticate: bearer JWT, else API key.
		token := bearerToken(r)
		if token == "" && r.URL.Path == positionsStreamPath {
			token = takeQueryToken(r)
		}

		var claims *auth.Claims
		switch {
		case token != "":
			c, err := validator.Validate(r.Context(), token)
			if err != nil {
				logger.Warn("token rejected", "error", err)
				writeProblem(w, r, http.StatusUnauthorized, "invalid-credential", "Invalid credential", "")
				return
			}
			claims = c
		case apiKey != "":
			c, err := keys.Verify(r.Context(), apiKey)
			switch {
			case errors.Is(err, auth.ErrInvalidAPIKey):
				writeProblem(w, r, http.StatusUnauthorized, "invalid-credential", "Invalid credential", "")
				return
			case err != nil:
				logger.Error("api key verification unavailable", "error", err)
				writeProblem(w, r, http.StatusServiceUnavailable, "auth-unavailable", "Authentication unavailable", "")
				return
			}
			claims = c
		default:
			writeProblem(w, r, http.StatusUnauthorized, "missing-credential", "Missing credential", "")
			return
		}

		// Step 3 — rate limit, per tenant per tier (limits.md §4).
		decision := lim.Allow(r.Context(), claims.TenantID, claims.Tier)
		if decision.FailedOpen {
			logger.Warn("rate limiter failed open — redis unreachable", "tenant", claims.TenantID)
		} else {
			h := w.Header()
			h.Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
			h.Set("X-RateLimit-Remaining", strconv.Itoa(int(decision.Remaining)))
			h.Set("X-RateLimit-Reset", strconv.FormatInt(decision.Reset.Unix(), 10))
		}
		if !decision.Allowed {
			retry := int(decision.RetryAfter.Seconds())
			if retry < 1 {
				retry = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			writeProblem(w, r, http.StatusTooManyRequests, "rate-limited", "Rate limit exceeded",
				"Tenant limit of "+strconv.Itoa(decision.Limit)+" requests/min exceeded.")
			return
		}

		// Step 4 — only now assert trusted context onto the outbound request.
		auth.SetAssertedContext(r, claims)

		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

// takeQueryToken reads ?token= and removes it from the request, so the credential
// is never forwarded to fleet-api, where it could end up in an access log.
func takeQueryToken(r *http.Request) string {
	q := r.URL.Query()
	token := q.Get("token")
	if q.Has("token") {
		q.Del("token")
		r.URL.RawQuery = q.Encode()
		r.RequestURI = r.URL.RequestURI()
	}
	return token
}
