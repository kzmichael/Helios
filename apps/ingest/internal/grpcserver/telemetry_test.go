package grpcserver

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/helios/trackline/apps/ingest/internal/backpressure"
	"github.com/helios/trackline/apps/ingest/internal/mqtt"
	"github.com/helios/trackline/apps/ingest/internal/ratelimit"
	"github.com/helios/trackline/apps/ingest/internal/registry"
)

func TestMemoryDeduperRemembersMarkedBatches(t *testing.T) {
	d := NewMemoryDeduper()
	ctx := context.Background()
	if d.Seen(ctx, "b1") {
		t.Fatal("an unseen batch must not read as a duplicate")
	}
	d.Mark(ctx, "b1")
	if !d.Seen(ctx, "b1") {
		t.Fatal("a marked batch must read as a duplicate")
	}
	if d.Seen(ctx, "b2") {
		t.Fatal("a different batch is not a duplicate")
	}
}

func TestMemoryDeduperIsSafeUnderConcurrency(t *testing.T) {
	d := NewMemoryDeduper()
	ctx := context.Background()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < 500; j++ {
				d.Mark(ctx, "b")
				d.Seen(ctx, "b")
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

func TestBucketAllowsBurstThenRefills(t *testing.T) {
	b := ratelimit.NewBucket(1, 3)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !b.Allow(now) {
			t.Fatalf("burst token %d should be allowed", i)
		}
	}
	if b.Allow(now) {
		t.Fatal("bucket should be empty after the burst")
	}
	if !b.Allow(now.Add(1100 * time.Millisecond)) {
		t.Fatal("one token should have refilled after a second")
	}
}

func TestTokenMatching(t *testing.T) {
	d := registry.Device{TokenHash: registry.HashToken("dev-token-abc")}
	if !d.TokenMatches("dev-token-abc") {
		t.Fatal("correct token should match")
	}
	if d.TokenMatches("dev-token-abd") || d.TokenMatches("") {
		t.Fatal("wrong or empty token must not match")
	}
	if (registry.Device{}).TokenMatches("anything") {
		t.Fatal("a device with no stored hash can never authenticate")
	}
}

func TestMQTTTopicParsing(t *testing.T) {
	if id, ok := mqtt.DeviceIDFromTopic("trackline/devices/de-1/telemetry"); !ok || id != "de-1" {
		t.Fatalf("expected de-1, got %q %v", id, ok)
	}
	for _, bad := range []string{"", "trackline/devices//telemetry", "other/devices/x/telemetry", "trackline/devices/x/y/telemetry", "trackline/devices/x/other"} {
		if _, ok := mqtt.DeviceIDFromTopic(bad); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}

func newTestServer(opts Options) *Server {
	if opts.Dedupe == nil {
		opts.Dedupe = NewMemoryDeduper()
	}
	return &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), opts: opts}
}

func TestThrottleEnforcesPerDeviceRate(t *testing.T) {
	s := newTestServer(Options{})
	adm := admission{deviceID: "d1", known: true, dev: registry.Device{Tier: "free", MaxHz: 1}}
	accepted := 0
	for i := 0; i < 20; i++ {
		if reason, _ := s.throttle(adm); reason == "" {
			accepted++
		}
	}
	if accepted != 5 { // the burst; nothing refills inside this loop
		t.Fatalf("expected the 5-point burst to pass, %d did", accepted)
	}
	if reason, retry := s.throttle(adm); reason != "rate limited" || retry != 1000 {
		t.Fatalf("expected a 1s retry hint, got %q %d", reason, retry)
	}
}

func TestThrottleShedsByTier(t *testing.T) {
	sh := backpressure.New(10)
	for i := 0; i < 10; i++ {
		sh.Begin() // backlog stays high
	}
	sh.Update(time.Now())
	sh.Update(time.Now())
	s := newTestServer(Options{Shedder: sh})

	free := admission{deviceID: "free-dev", known: true, dev: registry.Device{Tier: "free", MaxHz: 1000}}
	ent := admission{deviceID: "ent-dev", known: true, dev: registry.Device{Tier: "enterprise", MaxHz: 1000}}

	shedFree, shedEnt := 0, 0
	for i := 0; i < 5; i++ { // stay inside each device's burst so only shedding can reject
		if reason, _ := s.throttle(free); reason != "" {
			shedFree++
		}
		if reason, _ := s.throttle(ent); reason != "" {
			shedEnt++
		}
	}
	if shedFree == 0 {
		t.Fatal("free tier should be shed under sustained backlog")
	}
	if shedEnt != 0 {
		t.Fatal("enterprise must never be shed")
	}
}

func TestAdmitWithoutRegistryAcceptsEverything(t *testing.T) {
	s := newTestServer(Options{})
	if adm := s.admit(context.Background(), "", "any-device"); adm.reason != "" {
		t.Fatalf("a plain sink must accept, got %q", adm.reason)
	}
}
