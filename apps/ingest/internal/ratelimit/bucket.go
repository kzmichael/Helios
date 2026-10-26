// Package ratelimit is the per-device telemetry token bucket from limits.md §3.
// It is in-process on purpose: it protects ingest from a single chatty device,
// and the registry's per-tier rate is the only shared input.
package ratelimit

import (
	"sync"
	"time"
)

type Bucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	last     time.Time
}

// NewBucket allows a sustained ratePerSec with bursts up to burst points.
func NewBucket(ratePerSec, burst float64) *Bucket {
	return &Bucket{tokens: burst, capacity: burst, rate: ratePerSec}
}

// Allow takes one token if one is available.
func (b *Bucket) Allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.last.IsZero() {
		b.tokens += now.Sub(b.last).Seconds() * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
