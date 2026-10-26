package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORSPermitsAllowedPreflightWithoutCallingNext(t *testing.T) {
	called := false
	handler := withCORS([]string{"http://localhost:3000"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodOptions, "/v1/fleets", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("unexpected allow-origin header %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != corsAllowedHeaders {
		t.Fatalf("unexpected allow-headers header %q", got)
	}
	if called {
		t.Fatal("preflight unexpectedly reached the authenticated handler")
	}
}

func TestWithCORSRejectsDisallowedPreflight(t *testing.T) {
	handler := withCORS([]string{"http://localhost:3000"}, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodOptions, "/v1/fleets", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestWithCORSAddsHeaderToAllowedActualResponse(t *testing.T) {
	handler := withCORS([]string{"http://localhost:3000"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/v1/fleets", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected downstream status %d, got %d", http.StatusOK, response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("unexpected allow-origin header %q", got)
	}
}

func TestWithCORSRejectsUnsupportedPreflightHeaders(t *testing.T) {
	handler := withCORS([]string{"http://localhost:3000"}, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodOptions, "/v1/fleets", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "authorization, x-unapproved")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}
