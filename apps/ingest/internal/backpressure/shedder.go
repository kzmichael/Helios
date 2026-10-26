// Package backpressure implements the load-shedding order from
// apps/_contracts/limits.md §6. It watches ingest's own signals (publish backlog,
// publish latency, publish error rate) and decides, per tier, whether to accept a
// point, sample it, or turn it away with a retry_after.
//
// Enterprise telemetry is never shed.
package backpressure

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Action int

const (
	Accept Action = iota
	Sample        // keep 1 in SampleEvery points
	Reject
)

const (
	// Signal thresholds from limits.md §6.
	backlogFraction = 0.80
	publishP99      = 500 * time.Millisecond
	errorRate       = 0.10

	SampleEvery = 4
	window      = 30 * time.Second
)

type Shedder struct {
	capacity int64
	inflight atomic.Int64
	level    atomic.Int32
	counter  atomic.Uint64

	mu        sync.Mutex
	latencies [256]time.Duration
	nLat      int
	events    []outcome
	calmSince time.Time
}

type outcome struct {
	at time.Time
	ok bool
}

// New builds a shedder whose backlog signal is relative to capacity in-flight publishes.
func New(capacity int) *Shedder {
	if capacity <= 0 {
		capacity = 2000
	}
	return &Shedder{capacity: int64(capacity)}
}

// Begin marks a publish in flight; call the returned func with its result.
func (s *Shedder) Begin() func(err error) {
	s.inflight.Add(1)
	started := time.Now()
	return func(err error) {
		s.inflight.Add(-1)
		s.record(time.Since(started), err == nil)
	}
}

func (s *Shedder) record(d time.Duration, ok bool) {
	s.mu.Lock()
	s.latencies[s.nLat%len(s.latencies)] = d
	s.nLat++
	s.events = append(s.events, outcome{at: time.Now(), ok: ok})
	s.mu.Unlock()
}

// Pressure is the worst of the three signals, where 1.0 means "at threshold".
func (s *Shedder) Pressure(now time.Time) float64 {
	p := float64(s.inflight.Load()) / (float64(s.capacity) * backlogFraction)

	s.mu.Lock()
	defer s.mu.Unlock()

	n := s.nLat
	if n > len(s.latencies) {
		n = len(s.latencies)
	}
	if n >= 20 {
		sorted := append([]time.Duration(nil), s.latencies[:n]...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		if v := float64(sorted[(n*99)/100]) / float64(publishP99); v > p {
			p = v
		}
	}

	cutoff := now.Add(-window)
	kept := s.events[:0]
	failed := 0
	for _, e := range s.events {
		if e.at.After(cutoff) {
			kept = append(kept, e)
			if !e.ok {
				failed++
			}
		}
	}
	s.events = kept
	if len(kept) >= 20 {
		if v := (float64(failed) / float64(len(kept))) / errorRate; v > p {
			p = v
		}
	}
	return p
}

// LevelFor maps pressure to a shed level. Levels 1-4 follow limits.md: sample
// Free, reject Free, sample Pro, reject Pro. Level 5 (degrade reads) is not
// ingest's to apply.
func LevelFor(pressure float64) int {
	switch {
	case pressure < 1.0:
		return 0
	case pressure < 1.25:
		return 1
	case pressure < 1.5:
		return 2
	case pressure < 1.75:
		return 3
	default:
		return 4
	}
}

// Update recomputes the level. It escalates immediately but only relaxes after
// ten calm seconds, so the level does not flap around a threshold.
func (s *Shedder) Update(now time.Time) int {
	target := int32(LevelFor(s.Pressure(now)))
	cur := s.level.Load()

	switch {
	case target > cur:
		s.level.Store(target)
		s.calmSince = time.Time{}
	case target < cur:
		if s.calmSince.IsZero() {
			s.calmSince = now
		} else if now.Sub(s.calmSince) >= 10*time.Second {
			s.level.Store(cur - 1)
			s.calmSince = time.Time{}
		}
	default:
		s.calmSince = time.Time{}
	}
	return int(s.level.Load())
}

// Run updates the level once a second until stop is closed.
func (s *Shedder) Run(stop <-chan struct{}) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			s.Update(now)
		}
	}
}

func (s *Shedder) Level() int { return int(s.level.Load()) }

// Decide returns what to do with one point from a device on the given tier.
func (s *Shedder) Decide(tier string) Action {
	return Decide(s.Level(), tier, s.counter.Add(1))
}

// Decide is the pure shed policy: which tiers are touched at which level.
func Decide(level int, tier string, seq uint64) Action {
	switch tier {
	case "enterprise":
		return Accept
	case "pro":
		switch {
		case level >= 4:
			return Reject
		case level >= 3:
			return sampled(seq)
		}
	default: // free, and anything unrecognised
		switch {
		case level >= 2:
			return Reject
		case level >= 1:
			return sampled(seq)
		}
	}
	return Accept
}

func sampled(seq uint64) Action {
	if seq%SampleEvery == 0 {
		return Accept
	}
	return Sample
}
