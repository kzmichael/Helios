// Package registry is ingest's cache of provisioned devices, read from the Redis
// hash the pipeline's registry-sync role maintains. It keeps the hot path off
// Postgres: auth.md §5 says device tokens are verified against a cache, so
// revocation is eventually consistent, bounded by the TTL below.
package registry

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisKey    = "registry:devices"
	positiveTTL = 30 * time.Second
	negativeTTL = 5 * time.Second
)

type Device struct {
	ID        string
	Tenant    string  `json:"t"`
	Vehicle   string  `json:"v"`
	Tier      string  `json:"tier"`
	MaxHz     float64 `json:"hz"`
	TokenHash string  `json:"h"`
}

type entry struct {
	dev     Device
	found   bool
	fetched time.Time
}

type Cache struct {
	rdb *redis.Client

	mu      sync.RWMutex
	entries map[string]entry
}

func New(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb, entries: map[string]entry{}}
}

// Lookup resolves a device id. If Redis is unreachable it keeps serving the last
// answer it had rather than turning a registry outage into a telemetry outage.
func (c *Cache) Lookup(ctx context.Context, deviceID string) (Device, bool) {
	c.mu.RLock()
	e, ok := c.entries[deviceID]
	c.mu.RUnlock()

	if ok {
		ttl := positiveTTL
		if !e.found {
			ttl = negativeTTL
		}
		if time.Since(e.fetched) < ttl {
			return e.dev, e.found
		}
	}

	raw, err := c.rdb.HGet(ctx, redisKey, deviceID).Result()
	switch {
	case err == redis.Nil:
		c.store(deviceID, entry{found: false, fetched: time.Now()})
		return Device{}, false
	case err != nil:
		if ok {
			return e.dev, e.found // stale beats unavailable
		}
		return Device{}, false
	}

	var d Device
	if json.Unmarshal([]byte(raw), &d) != nil {
		return Device{}, false
	}
	d.ID = deviceID
	c.store(deviceID, entry{dev: d, found: true, fetched: time.Now()})
	return d, true
}

func (c *Cache) store(id string, e entry) {
	c.mu.Lock()
	c.entries[id] = e
	c.mu.Unlock()
}

// HashToken is the SHA-256 hex digest the registry stores (auth.md §5).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// TokenMatches compares a presented token with the device's stored hash in
// constant time.
func (d Device) TokenMatches(token string) bool {
	if token == "" || d.TokenHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(HashToken(token)), []byte(d.TokenHash)) == 1
}
