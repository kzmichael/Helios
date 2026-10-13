// Package proxy forwards authenticated, rate-limited requests to fleet-api.
//
// edge-gateway is the only trust boundary in the system (auth.md §3). By the
// time a request reaches NewFleetAPIProxy, it has already been authenticated
// and had its inbound X-Trackline-* headers stripped and replaced.
package proxy

import (
	"net/http/httputil"
	"net/url"
)

// NewFleetAPIProxy builds a reverse proxy to fleet-api. It does not touch
// headers — that is auth.StripInboundTrust and auth.SetAssertedContext's job,
// applied earlier in the middleware chain, not here.
func NewFleetAPIProxy(target string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(u), nil
}
