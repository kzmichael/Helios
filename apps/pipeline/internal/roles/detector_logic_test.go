package roles

import (
	"testing"
	"time"

	"github.com/helios/trackline/apps/pipeline/internal/geo"
	"github.com/helios/trackline/apps/pipeline/internal/registry"
)

var (
	t0  = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	dev = registry.Device{DeviceID: "d1", VehicleID: "v1", TenantID: "t1", FleetID: "f1", Registration: "KA-01", SpeedLimit: 70}

	// A 0.01 degree square around (12.00, 77.00).
	fence = registry.Geofence{
		ID: "g1", TenantID: "t1", Name: "Depot", Kind: "inclusion", DwellSecs: 120,
		Boundary: geo.Polygon{{76.995, 11.995}, {77.005, 11.995}, {77.005, 12.005}, {76.995, 12.005}, {76.995, 11.995}},
	}
)

func obs(offsetSecs int, lat, lon, speed float64) Observation {
	at := t0.Add(time.Duration(offsetSecs) * time.Second)
	return Observation{Dev: dev, Fences: []registry.Geofence{fence}, RecordedAt: at, IngestedAt: at,
		Lat: lat, Lon: lon, SpeedKph: speed, Ignition: true}
}

func kinds(evs []Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, string(e.Kind)+":"+e.Transition)
	}
	return out
}

func TestFirstPointInsideIsABaselineNotAnEnter(t *testing.T) {
	d := NewDetector()
	if evs := d.Observe(obs(0, 12.0, 77.0, 30)); len(evs) != 0 {
		t.Fatalf("first observation must not emit, got %v", kinds(evs))
	}
}

func TestEnterThenExit(t *testing.T) {
	d := NewDetector()
	d.Observe(obs(0, 12.0, 76.90, 30)) // outside baseline
	evs := d.Observe(obs(1, 12.0, 77.0, 30))
	if got := kinds(evs); len(got) != 1 || got[0] != "geofence:enter" {
		t.Fatalf("expected one enter, got %v", got)
	}
	evs = d.Observe(obs(2, 12.0, 77.1, 30))
	if got := kinds(evs); len(got) != 1 || got[0] != "geofence:exit" {
		t.Fatalf("expected one exit, got %v", got)
	}
}

func TestStartingInsideThenLeavingEmitsOnlyExit(t *testing.T) {
	d := NewDetector()
	d.Observe(obs(0, 12.0, 77.0, 30))
	evs := d.Observe(obs(1, 12.0, 77.1, 30))
	if got := kinds(evs); len(got) != 1 || got[0] != "geofence:exit" {
		t.Fatalf("expected one exit, got %v", got)
	}
}

func TestDwellEmitsOnceAfterThreshold(t *testing.T) {
	d := NewDetector()
	d.Observe(obs(0, 12.0, 76.90, 30))
	d.Observe(obs(1, 12.0, 77.0, 30)) // enter at t=1
	if evs := d.Observe(obs(100, 12.0, 77.0, 30)); len(evs) != 0 {
		t.Fatalf("no dwell before the threshold, got %v", kinds(evs))
	}
	evs := d.Observe(obs(125, 12.0, 77.0, 30))
	if got := kinds(evs); len(got) != 1 || got[0] != "geofence:dwell" {
		t.Fatalf("expected a dwell, got %v", got)
	}
	if evs := d.Observe(obs(200, 12.0, 77.0, 30)); len(evs) != 0 {
		t.Fatalf("dwell must fire once per visit, got %v", kinds(evs))
	}
}

func TestRedeliveredAndOutOfOrderPointsAreIgnored(t *testing.T) {
	d := NewDetector()
	d.Observe(obs(0, 12.0, 76.90, 30))
	if len(d.Observe(obs(1, 12.0, 77.0, 30))) != 1 {
		t.Fatal("first delivery should enter")
	}
	if evs := d.Observe(obs(1, 12.0, 77.0, 30)); len(evs) != 0 {
		t.Fatal("a redelivered point must not re-emit")
	}
	if evs := d.Observe(obs(0, 12.0, 76.90, 30)); len(evs) != 0 {
		t.Fatal("an older point must be ignored")
	}
}

func TestForwardSkewedClockIsClampedToIngestTime(t *testing.T) {
	recorded := t0.Add(10 * time.Minute)
	if got := EffectiveTime(recorded, t0); !got.Equal(t0) {
		t.Fatalf("expected clamp to ingest time, got %v", got)
	}
	if got := EffectiveTime(t0.Add(2*time.Second), t0); !got.Equal(t0.Add(2 * time.Second)) {
		t.Fatal("small drift should be left alone")
	}
}

func TestSpeedingNeedsSustainedExcessAndRepeats(t *testing.T) {
	d := NewDetector()
	var all []Event
	for s := 0; s <= 35; s++ {
		all = append(all, d.Observe(obs(s, 12.0, 76.90, 90))...) // limit is 70
	}
	var speeding []Event
	for _, e := range all {
		if e.Kind == KindSpeeding {
			speeding = append(speeding, e)
		}
	}
	if len(speeding) != 3 { // at ~10s, ~20s, ~30s
		t.Fatalf("expected 3 speeding events over 35s, got %d", len(speeding))
	}
	if speeding[0].DurationSecs < 10 || speeding[0].LimitKph != 70 || speeding[0].SpeedKph != 90 {
		t.Fatalf("unexpected first speeding event: %+v", speeding[0])
	}
}

func TestSlowingDownResetsSpeeding(t *testing.T) {
	d := NewDetector()
	for s := 0; s < 8; s++ {
		d.Observe(obs(s, 12.0, 76.90, 90))
	}
	d.Observe(obs(8, 12.0, 76.90, 40)) // back under the limit
	var n int
	for s := 9; s < 16; s++ {
		for _, e := range d.Observe(obs(s, 12.0, 76.90, 90)) {
			if e.Kind == KindSpeeding {
				n++
			}
		}
	}
	if n != 0 {
		t.Fatal("the over-limit clock must restart after slowing down")
	}
}

func TestIdleRequiresIgnitionOnAndStationary(t *testing.T) {
	d := NewDetector()
	var idle int
	for s := 0; s <= 130; s++ {
		for _, e := range d.Observe(obs(s, 12.0, 76.90, 0)) {
			if e.Kind == KindIdle {
				idle++
			}
		}
	}
	if idle != 2 { // at ~60s and ~120s
		t.Fatalf("expected 2 idle events, got %d", idle)
	}

	d = NewDetector()
	for s := 0; s <= 130; s++ {
		o := obs(s, 12.0, 76.90, 0)
		o.Ignition = false
		if evs := d.Observe(o); len(evs) != 0 {
			t.Fatal("ignition off is parked, not idling")
		}
	}
}

func TestOfflineSweepEmitsOnceAMinute(t *testing.T) {
	d := NewDetector()
	d.Observe(obs(0, 12.0, 76.90, 30))
	lookup := func(string) (registry.Device, bool) { return dev, true }

	now := time.Now()
	if evs := d.Sweep(now.Add(30*time.Second), lookup); len(evs) != 0 {
		t.Fatal("30s of silence is not offline")
	}
	evs := d.Sweep(now.Add(90*time.Second), lookup)
	if len(evs) != 1 || evs[0].Kind != KindOffline || evs[0].SilentSecs < 89 {
		t.Fatalf("expected one offline event, got %+v", evs)
	}
	if evs := d.Sweep(now.Add(100*time.Second), lookup); len(evs) != 0 {
		t.Fatal("offline repeats only once a minute")
	}
	if evs := d.Sweep(now.Add(160*time.Second), lookup); len(evs) != 1 {
		t.Fatal("offline should repeat after a minute")
	}
}

func TestReportingAgainClearsOffline(t *testing.T) {
	d := NewDetector()
	d.Observe(obs(0, 12.0, 76.90, 30))
	lookup := func(string) (registry.Device, bool) { return dev, true }
	d.Sweep(time.Now().Add(90*time.Second), lookup)
	d.Observe(obs(200, 12.0, 76.90, 30))
	if evs := d.Sweep(time.Now().Add(30*time.Second), lookup); len(evs) != 0 {
		t.Fatal("a vehicle that just reported is not offline")
	}
}

func TestSeedAdoptsRecentLastSeenOnly(t *testing.T) {
	d := NewDetector()
	recent := registry.Device{VehicleID: "recent", LastSeenAt: t0.Add(-time.Hour)}
	old := registry.Device{VehicleID: "old", LastSeenAt: t0.Add(-72 * time.Hour)}
	never := registry.Device{VehicleID: "never"}
	d.Seed([]registry.Device{recent, old, never}, t0)
	if len(d.states) != 1 || d.states["recent"] == nil {
		t.Fatalf("expected only the recent vehicle to be seeded, got %d states", len(d.states))
	}
}
