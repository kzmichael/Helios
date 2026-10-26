package grpcserver

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// dedupeWindow bounds how long a batch id is remembered.
const dedupeWindow = 10 * time.Minute

// Deduper remembers PublishBatch ids so a device retrying a batch is not ingested
// twice (kafka-topics.md §3: the system is at-least-once, so consumers and
// producers must tolerate repeats).
type Deduper interface {
	Seen(ctx context.Context, batchID string) bool
	Mark(ctx context.Context, batchID string)
}

// MemoryDeduper is per-process: a restart, or a second replica, forgets.
type MemoryDeduper struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func NewMemoryDeduper() *MemoryDeduper { return &MemoryDeduper{seen: map[string]time.Time{}} }

func (m *MemoryDeduper) Seen(_ context.Context, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evictLocked(time.Now())
	_, ok := m.seen[id]
	return ok
}

func (m *MemoryDeduper) Mark(_ context.Context, id string) {
	m.mu.Lock()
	m.seen[id] = time.Now()
	m.mu.Unlock()
}

func (m *MemoryDeduper) evictLocked(now time.Time) {
	cutoff := now.Add(-dedupeWindow)
	for id, at := range m.seen {
		if at.Before(cutoff) {
			delete(m.seen, id)
		}
	}
}

// RedisDeduper is shared across replicas and survives restarts. If Redis is
// unavailable it falls back to the in-process map instead of failing ingestion.
type RedisDeduper struct {
	rdb      *redis.Client
	fallback *MemoryDeduper
}

func NewRedisDeduper(rdb *redis.Client) *RedisDeduper {
	return &RedisDeduper{rdb: rdb, fallback: NewMemoryDeduper()}
}

func (r *RedisDeduper) Seen(ctx context.Context, id string) bool {
	n, err := r.rdb.Exists(ctx, "ingest:batch:"+id).Result()
	if err != nil {
		return r.fallback.Seen(ctx, id)
	}
	return n > 0
}

func (r *RedisDeduper) Mark(ctx context.Context, id string) {
	if err := r.rdb.Set(ctx, "ingest:batch:"+id, 1, dedupeWindow).Err(); err != nil {
		r.fallback.Mark(ctx, id)
	}
}
