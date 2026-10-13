// Package ratelimit implements per-tenant request-rate limiting per
// apps/_contracts/limits.md §4.
//
// Token bucket, not fixed-window or sliding-log: it permits short bursts without
// permitting sustained overload, and it's the algorithm limits.md specifies.
//
// The failure mode matters as much as the happy path: if Redis is unreachable,
// this limiter fails OPEN (serves the request). Rejecting all traffic because
// the limiter itself is down converts a degradation into an outage — see
// limits.md §4. That decision must be alerted on, not silently relied upon.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tier limits from apps/_contracts/limits.md §2. Burst is expressed as extra
// tokens available for the burst window, not a multiplier — simpler to reason
// about in the Lua script below.
type TierLimit struct {
	RequestsPerMinute int
	BurstTokens       int
}

var TierLimits = map[string]TierLimit{
	"free":       {RequestsPerMinute: 60, BurstTokens: 60},
	"pro":        {RequestsPerMinute: 600, BurstTokens: 600},
	"enterprise": {RequestsPerMinute: 6000, BurstTokens: 6000},
}

// tokenBucketScript performs check-and-decrement atomically so concurrent
// requests from the same tenant cannot race past the limit. KEYS[1] is the
// bucket key; ARGV: capacity, refill_per_sec, now_ms, cost.
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_per_sec = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil then
  tokens = capacity
  ts = now_ms
end

local elapsed = math.max(0, now_ms - ts) / 1000
tokens = math.min(capacity, tokens + elapsed * refill_per_sec)

local allowed = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
end

redis.call("HMSET", key, "tokens", tokens, "ts", now_ms)
redis.call("EXPIRE", key, 120)

return {allowed, tokens}
`

type Decision struct {
	Allowed   bool
	Remaining float64
	// FailedOpen is true when Redis was unreachable and the request was
	// allowed as a deliberate availability-over-fairness trade-off.
	FailedOpen bool
}

type Limiter struct {
	rdb    *redis.Client
	script *redis.Script
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb, script: redis.NewScript(tokenBucketScript)}
}

// Allow checks and decrements one request-cost token from tenantID's bucket for
// tier. On any Redis error it fails open — see the package doc comment.
func (l *Limiter) Allow(ctx context.Context, tenantID, tier string) Decision {
	limit, ok := TierLimits[tier]
	if !ok {
		limit = TierLimits["free"] // unknown tier: fail toward the most restrictive
	}

	key := fmt.Sprintf("ratelimit:%s:api", tenantID)
	refillPerSec := float64(limit.RequestsPerMinute) / 60.0
	now := time.Now().UnixMilli()

	res, err := l.script.Run(ctx, l.rdb, []string{key},
		limit.BurstTokens, refillPerSec, now, 1).Result()
	if err != nil {
		// Deliberate: Redis down does not mean the platform goes down for
		// every tenant. This must be paired with an alert in production —
		// a silently-disabled limiter is how one tenant takes down everyone.
		return Decision{Allowed: true, FailedOpen: true}
	}

	values, ok := res.([]interface{})
	if !ok || len(values) != 2 {
		return Decision{Allowed: true, FailedOpen: true}
	}

	allowed, _ := values[0].(int64)
	remaining, _ := values[1].(float64)

	return Decision{Allowed: allowed == 1, Remaining: remaining}
}
