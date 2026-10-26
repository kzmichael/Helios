package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSplitAPIKey(t *testing.T) {
	good := map[string][2]string{
		"tl_dev_abcd1234.dev-secret-x": {"tl_dev_abcd1234", "dev-secret-x"},
		"a.b.c":                        {"a", "b.c"},
	}
	for in, want := range good {
		p, s, ok := splitAPIKey(in)
		if !ok || p != want[0] || s != want[1] {
			t.Errorf("%q -> %q %q %v", in, p, s, ok)
		}
	}
	for _, bad := range []string{"", "nodot", ".secret", "prefix.", "pre fix.secret", "../x.secret"} {
		if _, _, ok := splitAPIKey(bad); ok {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestVerifierSendsOnlyTheHashAndCaches(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["secretHash"] != hashHex("the-secret") || body["prefix"] != "tl_dev_1" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		if _, leaked := body["secret"]; leaked {
			t.Error("the raw secret must never be sent")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tenantId": "t1", "tier": "pro", "subject": "apikey:tl_dev_1", "roles": []string{"operator"}})
	}))
	defer srv.Close()

	v := NewAPIKeyVerifier(srv.URL)
	for i := 0; i < 3; i++ {
		c, err := v.Verify(context.Background(), "tl_dev_1.the-secret")
		if err != nil || c.TenantID != "t1" || c.Subject != "apikey:tl_dev_1" {
			t.Fatalf("verify: %v %+v", err, c)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one upstream call, got %d", calls.Load())
	}

	if _, err := v.Verify(context.Background(), "tl_dev_1.wrong"); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("wrong secret: %v", err)
	}
}

func TestVerifierOutageIsNotSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := NewAPIKeyVerifier(srv.URL).Verify(context.Background(), "tl_dev_1.s"); !errors.Is(err, ErrVerifierDown) {
		t.Fatalf("expected ErrVerifierDown, got %v", err)
	}

	srv.Close()
	if _, err := NewAPIKeyVerifier(srv.URL).Verify(context.Background(), "tl_dev_1.s"); !errors.Is(err, ErrVerifierDown) {
		t.Fatalf("unreachable upstream: %v", err)
	}
}
