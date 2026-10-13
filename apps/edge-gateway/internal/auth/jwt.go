// Package auth validates access tokens per apps/_contracts/auth.md §2.
//
// Every check listed there is mandatory and is implemented here. None of them are
// optimisations you can skip: skipping any one is a vulnerability.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims mirrors the token shape documented in auth.md §2.
type Claims struct {
	jwt.RegisteredClaims
	TenantID string `json:"tid"`
	Tier     string `json:"tier"`
	Roles    []string `json:"roles"`
	Scope    string `json:"scp"`
}

var (
	ErrNoKeyID       = errors.New("auth: token has no kid header")
	ErrUnknownKey    = errors.New("auth: no matching JWKS key")
	ErrBadAlgorithm  = errors.New("auth: unexpected signing algorithm")
	ErrMissingTenant = errors.New("auth: token has no tenant id")
)

// jwk is the subset of RFC 7517 fields this validator needs. Only RSA keys are
// supported — auth.md mandates rejecting HMAC algorithms outright.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

// Validator fetches and caches a JWKS document, validating tokens against it.
// A JWKS fetch failure must never cause a token to be accepted unvalidated —
// see auth.md §2, "Validation performed by edge-gateway", point 1.
type Validator struct {
	jwksURL string
	issuer  string
	aud     string
	skew    time.Duration
	ttl     time.Duration

	client *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsaPublicKeyMaterial
	fetchedAt time.Time
}

type rsaPublicKeyMaterial struct {
	n *big.Int
	e int
}

func NewValidator(jwksURL, issuer, aud string, ttl, skew time.Duration) *Validator {
	return &Validator{
		jwksURL: jwksURL,
		issuer:  issuer,
		aud:     aud,
		skew:    skew,
		ttl:     ttl,
		client:  &http.Client{Timeout: 5 * time.Second},
		keys:    map[string]*rsaPublicKeyMaterial{},
	}
}

// Validate performs every check mandated by auth.md §2. It never accepts a
// token if the JWKS could not be resolved.
func (v *Validator) Validate(ctx context.Context, tokenString string) (*Claims, error) {
	claims := &Claims{}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))

	_, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		// Reject "none" and any HMAC algorithm outright — WithValidMethods above
		// already refuses anything but RS256, this is defence in depth.
		if t.Method.Alg() != "RS256" {
			return nil, ErrBadAlgorithm
		}

		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, ErrNoKeyID
		}

		key, err := v.keyFor(ctx, kid)
		if err != nil {
			return nil, err // JWKS unresolved => reject, never accept unvalidated.
		}
		return (&rsaPublicKey{n: key.n, e: key.e}).toStdlib(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: token invalid: %w", err)
	}

	if claims.Issuer != v.issuer {
		return nil, fmt.Errorf("auth: unexpected issuer %q", claims.Issuer)
	}
	if !audienceContains(claims.Audience, v.aud) {
		return nil, fmt.Errorf("auth: token not intended for %q", v.aud)
	}
	if claims.TenantID == "" {
		return nil, ErrMissingTenant
	}

	return claims, nil
}

func audienceContains(aud jwt.ClaimStrings, want string) bool {
	for _, a := range aud {
		if a == want {
			return true
		}
	}
	return false
}

// keyFor returns the RSA key material for kid, refreshing the JWKS cache if the
// key is unknown or the cache has expired. Refresh failures propagate as errors
// rather than falling back to stale-but-unverifiable trust.
func (v *Validator) keyFor(ctx context.Context, kid string) (*rsaPublicKeyMaterial, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	fresh := time.Since(v.fetchedAt) < v.ttl
	v.mu.RUnlock()

	if ok && fresh {
		return key, nil
	}

	if err := v.refresh(ctx); err != nil {
		return nil, fmt.Errorf("auth: jwks refresh failed: %w", err)
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, ErrUnknownKey
	}
	return key, nil
}

func (v *Validator) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}

	next := map[string]*rsaPublicKeyMaterial{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, e, err := decodeRSAComponents(k.N, k.E)
		if err != nil {
			continue // skip a malformed key rather than fail the whole refresh
		}
		next[k.Kid] = &rsaPublicKeyMaterial{n: n, e: e}
	}

	v.mu.Lock()
	v.keys = next
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}
