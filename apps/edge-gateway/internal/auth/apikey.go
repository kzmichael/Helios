package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Machine clients authenticate with an API key (auth.md §1): X-Trackline-Key is
// "<prefix>.<secret>". Only SHA-256(secret) is ever stored or sent anywhere — the
// gateway hashes the secret itself and asks fleet-api, which owns the api_key
// table, whether that prefix+hash is a live key and which tenant it belongs to.
//
// Revocation is therefore eventually consistent, bounded by the TTLs below.
const (
	apiKeyPositiveTTL = 30 * time.Second
	apiKeyNegativeTTL = 5 * time.Second
	apiKeyMaxLen      = 256
)

var (
	ErrInvalidAPIKey = errors.New("auth: invalid api key")
	ErrVerifierDown  = errors.New("auth: api key verifier unavailable")
)

type apiKeyPrincipal struct {
	TenantID string   `json:"tenantId"`
	Tier     string   `json:"tier"`
	Subject  string   `json:"subject"`
	Roles    []string `json:"roles"`
}

type apiKeyEntry struct {
	claims  *Claims
	fetched time.Time
}

// APIKeyVerifier checks API keys against fleet-api's internal endpoint.
type APIKeyVerifier struct {
	endpoint string
	client   *http.Client

	mu    sync.Mutex
	cache map[string]apiKeyEntry // sha256(full key) -> result (nil claims = known bad)
}

func NewAPIKeyVerifier(fleetAPIURL string) *APIKeyVerifier {
	return &APIKeyVerifier{
		endpoint: strings.TrimRight(fleetAPIURL, "/") + "/internal/api-keys/verify",
		client:   &http.Client{Timeout: 3 * time.Second},
		cache:    map[string]apiKeyEntry{},
	}
}

// splitAPIKey returns the prefix and secret of a well-formed key.
func splitAPIKey(key string) (prefix, secret string, ok bool) {
	if len(key) == 0 || len(key) > apiKeyMaxLen {
		return "", "", false
	}
	prefix, secret, found := strings.Cut(key, ".")
	if !found || prefix == "" || secret == "" {
		return "", "", false
	}
	for _, r := range prefix {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return "", "", false
		}
	}
	return prefix, secret, true
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Verify returns the principal for a valid key, ErrInvalidAPIKey for a bad,
// expired or revoked one, and ErrVerifierDown if fleet-api cannot be asked
// (the caller must not treat that as success).
func (v *APIKeyVerifier) Verify(ctx context.Context, key string) (*Claims, error) {
	prefix, secret, ok := splitAPIKey(key)
	if !ok {
		return nil, ErrInvalidAPIKey
	}

	cacheKey := hashHex(key)
	v.mu.Lock()
	if e, hit := v.cache[cacheKey]; hit {
		ttl := apiKeyPositiveTTL
		if e.claims == nil {
			ttl = apiKeyNegativeTTL
		}
		if time.Since(e.fetched) < ttl {
			v.mu.Unlock()
			if e.claims == nil {
				return nil, ErrInvalidAPIKey
			}
			c := *e.claims
			return &c, nil
		}
	}
	v.mu.Unlock()

	body, _ := json.Marshal(map[string]string{"prefix": prefix, "secretHash": hashHex(secret)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, ErrVerifierDown
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVerifierDown, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized:
		v.store(cacheKey, nil)
		return nil, ErrInvalidAPIKey
	case resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated:
		return nil, fmt.Errorf("%w: status %d", ErrVerifierDown, resp.StatusCode)
	}

	var p apiKeyPrincipal
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&p); err != nil || p.TenantID == "" {
		return nil, fmt.Errorf("%w: unreadable response", ErrVerifierDown)
	}

	claims := &Claims{TenantID: p.TenantID, Tier: p.Tier, Roles: p.Roles}
	claims.Subject = p.Subject
	v.store(cacheKey, claims)
	return claims, nil
}

func (v *APIKeyVerifier) store(k string, c *Claims) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.cache) > 4096 { // bound memory against a stream of random keys
		v.cache = map[string]apiKeyEntry{}
	}
	v.cache[k] = apiKeyEntry{claims: c, fetched: time.Now()}
}
