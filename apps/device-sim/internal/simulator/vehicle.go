// Package simulator drives one simulated vehicle: it moves a position forward
// tick by tick according to a behaviour profile, and streams the result to
// ingest over the TelemetryIngest gRPC service.
package simulator

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/helios/trackline/apps/device-sim/internal/chaos"
	"github.com/helios/trackline/apps/device-sim/internal/identity"
	"github.com/helios/trackline/apps/device-sim/internal/profiles"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// ErrInjectedDisconnect is returned by StreamTo when chaos closes the stream.
var ErrInjectedDisconnect = errors.New("chaos: injected disconnect")

type Vehicle struct {
	DeviceID string
	Token    string
	Profile  profiles.Profile

	// EmissionHz overrides the profile's rate when positive.
	EmissionHz float64

	chaos *chaos.Injector

	// pauseUntil is a unix-nano deadline set from the server's retry_after_ms.
	pauseUntil atomic.Int64

	lat, lon   float64
	speed      float64
	heading    float64
	ignitionOn bool
	odometerKm float64
	rng        *rand.Rand
}

// NewVehicle starts a vehicle scattered ~2.5 km around its identity's start position.
func NewVehicle(id identity.Identity, p profiles.Profile, seed int64, inj *chaos.Injector) *Vehicle {
	return &Vehicle{
		DeviceID: id.DeviceID,
		Token:    id.Token,
		Profile:  p,
		chaos:    inj,
		lat:      id.Lat + (rand.New(rand.NewSource(seed)).Float64()-0.5)*0.05,
		lon:      id.Lon + (rand.New(rand.NewSource(seed+1)).Float64()-0.5)*0.05,
		speed:    p.SpeedKph.Cruise,
		heading:  rand.New(rand.NewSource(seed)).Float64() * 360,
		rng:      rand.New(rand.NewSource(seed)),
	}
}

// Tick advances this vehicle by one emission interval and returns the resulting
// telemetry point. Movement is a simple dead-reckoning model — good enough to
// exercise geofences and speed rules, not a routing engine.
func (v *Vehicle) Tick(intervalSeconds float64) *tracklinev1.TelemetryPoint {
	p := v.Profile

	if v.rng.Float64() < p.IgnitionOffProbability {
		v.ignitionOn = false
		v.speed = 0
	} else {
		v.ignitionOn = true
		if v.rng.Float64() < p.IdleProbability {
			v.speed = math.Max(0, v.speed*0.7)
		} else {
			target := p.SpeedKph.Cruise + (v.rng.Float64()-0.5)*p.SpeedKph.Cruise*0.3
			v.speed = clamp(target, p.SpeedKph.Min, p.SpeedKph.Max)
		}
	}

	v.heading += (v.rng.Float64() - 0.5) * 2 * p.HeadingChangeDegPerTick
	v.heading = math.Mod(v.heading+360, 360)

	if v.speed > 0 {
		distanceKm := (v.speed * intervalSeconds) / 3600.0
		v.odometerKm += distanceKm
		v.lat, v.lon = Advance(v.lat, v.lon, v.heading, distanceKm)
	}

	noisyLat := v.lat + (v.rng.Float64()-0.5)*2*metresToDegrees(p.GPSNoiseM)
	noisyLon := v.lon + (v.rng.Float64()-0.5)*2*metresToDegrees(p.GPSNoiseM)

	recordedAt := time.Now()
	if v.chaos != nil {
		recordedAt = recordedAt.Add(v.chaos.Skew())
	}

	return &tracklinev1.TelemetryPoint{
		DeviceId:     v.DeviceID,
		RecordedAt:   recordedAt.UnixMilli(),
		Position:     &tracklinev1.GeoPoint{Latitude: noisyLat, Longitude: noisyLon},
		SpeedKph:     v.speed,
		HeadingDeg:   v.heading,
		AccuracyM:    p.GPSNoiseM,
		OdometerKm:   v.odometerKm,
		FuelLevelPct: math.Max(0, 100-v.odometerKm*0.02),
		EngineTempC:  ifThenElse(v.ignitionOn, 85+v.rng.Float64()*10, 20),
		IgnitionOn:   v.ignitionOn,
	}
}

// Advance moves a position distanceKm along headingDeg (0 = north, clockwise).
func Advance(lat, lon, headingDeg, distanceKm float64) (float64, float64) {
	dLat := distanceKm / 111.0 * math.Cos(toRadians(headingDeg))
	dLon := distanceKm / (111.0 * math.Cos(toRadians(lat))) * math.Sin(toRadians(headingDeg))
	return lat + dLat, lon + dLon
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func toRadians(deg float64) float64 { return deg * math.Pi / 180 }

// metresToDegrees is a rough approximation adequate for simulated GPS noise —
// not for anything that needs geodetic precision.
func metresToDegrees(m float64) float64 { return m / 111_000.0 }

func ifThenElse(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

// Pace records the server's retry_after_ms so the next emissions wait it out. A
// well-behaved device slows down instead of retrying harder.
func (v *Vehicle) Pace(retryAfterMs uint32) {
	if retryAfterMs == 0 {
		return
	}
	v.pauseUntil.Store(time.Now().Add(time.Duration(retryAfterMs) * time.Millisecond).UnixNano())
}

func (v *Vehicle) interval() time.Duration {
	hz := v.Profile.EmissionHz
	if v.EmissionHz > 0 {
		hz = v.EmissionHz
	}
	return time.Duration(float64(time.Second) / hz)
}

// StreamTo runs this vehicle's tick loop against a gRPC stream until ctx is
// cancelled or an injected disconnect fires. The caller owns the stream's
// lifecycle (connect/reconnect); this function only produces points.
func (v *Vehicle) StreamTo(ctx context.Context, send func(*tracklinev1.TelemetryPoint) error) error {
	interval := v.interval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var disconnect <-chan time.Time
	if v.chaos != nil {
		if d := v.chaos.NextDisconnect(); d > 0 {
			t := time.NewTimer(d)
			defer t.Stop()
			disconnect = t.C
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-disconnect:
			return ErrInjectedDisconnect
		case <-ticker.C:
			if time.Now().UnixNano() < v.pauseUntil.Load() {
				continue
			}
			point := v.Tick(interval.Seconds())
			if v.chaos != nil && v.chaos.ShouldDrop() {
				continue
			}
			if err := send(point); err != nil {
				return err
			}
			if v.chaos != nil && v.chaos.ShouldDuplicate() {
				if err := send(point); err != nil {
					return err
				}
			}
		}
	}
}
