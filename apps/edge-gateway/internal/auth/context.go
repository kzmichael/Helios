// Header propagation per apps/_contracts/auth.md §3.
//
// The rule that matters most in this whole service lives in Middleware below:
// every inbound X-Trackline-* header is stripped before the gateway sets its own.
// Without that, a client can forge X-Trackline-Tenant and read another tenant's
// data once the request reaches fleet-api. That is a confused-deputy attack and
// the most likely way this system gets breached — see auth.md §3.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	HeaderTenant    = "X-Trackline-Tenant"
	HeaderSubject   = "X-Trackline-Subject"
	HeaderRoles     = "X-Trackline-Roles"
	HeaderRequestID = "X-Trackline-Request-Id"
)

// strippedPrefixes lists every header namespace a client must never be able to
// set directly. Extend this list before adding any new X-Trackline-* header.
var strippedPrefixes = []string{"X-Trackline-"}

// StripInboundTrust removes any header a client could use to impersonate the
// gateway's own trust assertions. Call this before Validate, and call Validate
// before SetAssertedContext — order matters.
func StripInboundTrust(r *http.Request) {
	for name := range r.Header {
		for _, prefix := range strippedPrefixes {
			if strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
				r.Header.Del(name)
				break
			}
		}
	}
}

// SetAssertedContext writes the gateway's own trust assertions onto the
// outbound (proxied) request. Only edge-gateway may call this, and only after
// StripInboundTrust and a successful Validate.
func SetAssertedContext(r *http.Request, claims *Claims) {
	r.Header.Set(HeaderTenant, claims.TenantID)
	r.Header.Set(HeaderSubject, claims.Subject)
	r.Header.Set(HeaderRoles, strings.Join(claims.Roles, ","))
	if r.Header.Get(HeaderRequestID) == "" {
		r.Header.Set(HeaderRequestID, newRequestID())
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
