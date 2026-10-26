package main

import (
	"net/http"
	"strings"
)

const (
	corsAllowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Authorization, Content-Type"
)

var allowedCORSMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodPatch:   {},
	http.MethodDelete:  {},
	http.MethodOptions: {},
}

var allowedCORSHeaders = map[string]struct{}{
	"authorization": {},
	"content-type":  {},
}

func withCORS(allowedOrigins []string, next http.Handler) http.Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins[origin] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
		}

		requestedMethod := r.Header.Get("Access-Control-Request-Method")
		if r.Method == http.MethodOptions && requestedMethod != "" {
			if _, ok := origins[origin]; !ok || !isAllowedCORSMethod(requestedMethod) || !hasAllowedCORSHeaders(r.Header.Get("Access-Control-Request-Headers")) {
				http.Error(w, "CORS preflight denied", http.StatusForbidden)
				return
			}

			setCORSHeaders(w, origin)
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if _, ok := origins[origin]; origin != "" && ok {
			setCORSHeaders(w, origin)
		}
		next.ServeHTTP(w, r)
	})
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
}

func isAllowedCORSMethod(method string) bool {
	_, ok := allowedCORSMethods[strings.ToUpper(strings.TrimSpace(method))]
	return ok
}

func hasAllowedCORSHeaders(headers string) bool {
	for _, header := range strings.Split(headers, ",") {
		if header = strings.ToLower(strings.TrimSpace(header)); header != "" {
			if _, ok := allowedCORSHeaders[header]; !ok {
				return false
			}
		}
	}
	return true
}
