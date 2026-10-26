package roles

import (
	"fmt"
	"time"

	"github.com/helios/trackline/apps/pipeline/internal/registry"
)

// Thresholds for the base events the detector emits. They are deliberately lower
// than any tenant's rule so that rule parameters (tolerance, minimum duration,
// idle and offline thresholds) are applied by the alert engine, not baked in here.
const (
	speedingMinDuration = 10 * time.Second
	speedingRepeat      = 10 * time.Second
	idleSpeedKph        = 2.0
	idleMinDuration     = 60 * time.Second
	idleRepeat          = 60 * time.Second
	offlineAfter        = 60 * time.Second
	offlineRepeat       = 60 * time.Second
	// A device clock further ahead than this is clamped to ingest time, so a
	// forward-skewed device cannot freeze its own "latest" position.
	maxFutureSkew = 5 * time.Second
)

type Observation struct {
	Dev        registry.Device
	Fences     []registry.Geofence
	RecordedAt time.Time
	IngestedAt time.Time
	Lat, Lon   float64
	SpeedKph   float64
	Ignition   bool
}

type EventKind string

const (
	KindGeofence EventKind = "geofence"
	KindSpeeding EventKind = "speeding"
	KindIdle     EventKind = "idle"
	KindOffline  EventKind = "offline"
)

// Event is a detector output, before it is encoded for Kafka.
type Event struct {
	Kind EventKind
	Dev  registry.Device
	At   time.Time
	Lat  float64
	Lon  float64
	Key  string

	Geofence   registry.Geofence
	Transition string // enter | exit | dwell
	DwellSecs  int64

	SpeedKph     float64
	LimitKph     float64
	DurationSecs int64

	IdleSecs int64

	SilentSecs int64
	LastSeen   time.Time
}

type fenceState struct {
	since        time.Time
	dwellEmitted bool
}

type vehicleState struct {
	lastRecorded time.Time
	lastSeen     time.Time

	baselined bool
	inside    map[string]*fenceState

	overSince        time.Time
	lastSpeedingEmit time.Time

	idleSince    time.Time
	lastIdleEmit time.Time

	lastOfflineEmit time.Time
}

// Detector holds per-vehicle state and turns observations into events. It is not
// safe for concurrent use; one goroutine owns it.
type Detector struct {
	states map[string]*vehicleState
}

func NewDetector() *Detector { return &Detector{states: map[string]*vehicleState{}} }

// EffectiveTime orders points by device time but never trusts a clock that runs
// ahead of ingest.
func EffectiveTime(recorded, ingested time.Time) time.Time {
	if !ingested.IsZero() && recorded.After(ingested.Add(maxFutureSkew)) {
		return ingested
	}
	return recorded
}

// Observe folds one telemetry point into the vehicle's state and returns any
// events it triggers. Points not newer than the last one are ignored, which also
// makes redelivered and duplicated points harmless.
func (d *Detector) Observe(o Observation) []Event {
	at := EffectiveTime(o.RecordedAt, o.IngestedAt)
	st, ok := d.states[o.Dev.VehicleID]
	if !ok {
		st = &vehicleState{inside: map[string]*fenceState{}}
		d.states[o.Dev.VehicleID] = st
	}
	if !st.lastRecorded.IsZero() && !at.After(st.lastRecorded) {
		return nil
	}
	st.lastRecorded = at
	st.lastSeen = time.Now()
	st.lastOfflineEmit = time.Time{}

	var out []Event
	out = append(out, d.geofences(st, o, at)...)
	out = append(out, d.speeding(st, o, at)...)
	out = append(out, d.idle(st, o, at)...)
	st.baselined = true
	return out
}

func (d *Detector) geofences(st *vehicleState, o Observation, at time.Time) []Event {
	var out []Event
	current := map[string]bool{}

	for _, g := range o.Fences {
		current[g.ID] = true
		in := g.Boundary.Contains(o.Lat, o.Lon)
		fs, was := st.inside[g.ID]

		switch {
		case in && !was:
			st.inside[g.ID] = &fenceState{since: at}
			// The first point ever seen for a vehicle is a baseline, not a transition:
			// we do not know where it was before.
			if st.baselined {
				out = append(out, d.fenceEvent(o, g, "enter", at, 0))
			}
		case !in && was:
			delete(st.inside, g.ID)
			out = append(out, d.fenceEvent(o, g, "exit", at, int64(at.Sub(fs.since).Seconds())))
		case in && was && !fs.dwellEmitted && g.DwellSecs > 0 && at.Sub(fs.since) >= time.Duration(g.DwellSecs)*time.Second:
			fs.dwellEmitted = true
			out = append(out, d.fenceEvent(o, g, "dwell", at, int64(at.Sub(fs.since).Seconds())))
		}
	}
	for id := range st.inside {
		if !current[id] {
			delete(st.inside, id) // geofence removed or disabled while inside
		}
	}
	return out
}

func (d *Detector) fenceEvent(o Observation, g registry.Geofence, transition string, at time.Time, dwell int64) Event {
	return Event{
		Kind: KindGeofence, Dev: o.Dev, At: at, Lat: o.Lat, Lon: o.Lon,
		Geofence: g, Transition: transition, DwellSecs: dwell,
		Key: fmt.Sprintf("geofence:%s:%s:%s:%d", o.Dev.VehicleID, g.ID, transition, at.UnixMilli()),
	}
}

func (d *Detector) speeding(st *vehicleState, o Observation, at time.Time) []Event {
	if o.SpeedKph <= o.Dev.SpeedLimit {
		st.overSince, st.lastSpeedingEmit = time.Time{}, time.Time{}
		return nil
	}
	if st.overSince.IsZero() {
		st.overSince = at
	}
	over := at.Sub(st.overSince)
	if over < speedingMinDuration {
		return nil
	}
	if !st.lastSpeedingEmit.IsZero() && at.Sub(st.lastSpeedingEmit) < speedingRepeat {
		return nil
	}
	st.lastSpeedingEmit = at
	return []Event{{
		Kind: KindSpeeding, Dev: o.Dev, At: at, Lat: o.Lat, Lon: o.Lon,
		SpeedKph: o.SpeedKph, LimitKph: o.Dev.SpeedLimit, DurationSecs: int64(over.Seconds()),
		Key: fmt.Sprintf("speeding:%s:%d", o.Dev.VehicleID, at.Unix()/int64(speedingRepeat.Seconds())),
	}}
}

func (d *Detector) idle(st *vehicleState, o Observation, at time.Time) []Event {
	if !o.Ignition || o.SpeedKph >= idleSpeedKph {
		st.idleSince, st.lastIdleEmit = time.Time{}, time.Time{}
		return nil
	}
	if st.idleSince.IsZero() {
		st.idleSince = at
	}
	idle := at.Sub(st.idleSince)
	if idle < idleMinDuration {
		return nil
	}
	if !st.lastIdleEmit.IsZero() && at.Sub(st.lastIdleEmit) < idleRepeat {
		return nil
	}
	st.lastIdleEmit = at
	return []Event{{
		Kind: KindIdle, Dev: o.Dev, At: at, Lat: o.Lat, Lon: o.Lon, IdleSecs: int64(idle.Seconds()),
		Key: fmt.Sprintf("idle:%s:%d", o.Dev.VehicleID, at.Unix()/int64(idleRepeat.Seconds())),
	}}
}

// Seed adopts last-seen times from the database for vehicles this process has not
// heard from yet, so a restart does not forget that a vehicle was already silent.
func (d *Detector) Seed(devices []registry.Device, now time.Time) {
	for _, dev := range devices {
		if _, ok := d.states[dev.VehicleID]; ok || dev.LastSeenAt.Year() < 2000 {
			continue
		}
		if now.Sub(dev.LastSeenAt) > 24*time.Hour {
			continue
		}
		d.states[dev.VehicleID] = &vehicleState{inside: map[string]*fenceState{}, lastSeen: dev.LastSeenAt}
	}
}

// Sweep emits offline events for vehicles that have gone quiet. lookup resolves a
// vehicle id to its device record.
func (d *Detector) Sweep(now time.Time, lookup func(vehicleID string) (registry.Device, bool)) []Event {
	var out []Event
	for vehicleID, st := range d.states {
		if st.lastSeen.IsZero() {
			continue
		}
		silent := now.Sub(st.lastSeen)
		if silent < offlineAfter {
			continue
		}
		if !st.lastOfflineEmit.IsZero() && now.Sub(st.lastOfflineEmit) < offlineRepeat {
			continue
		}
		dev, ok := lookup(vehicleID)
		if !ok {
			continue
		}
		st.lastOfflineEmit = now
		out = append(out, Event{
			Kind: KindOffline, Dev: dev, At: now, SilentSecs: int64(silent.Seconds()), LastSeen: st.lastSeen,
			Key: fmt.Sprintf("offline:%s:%d", vehicleID, now.Unix()/int64(offlineRepeat.Seconds())),
		})
	}
	return out
}
