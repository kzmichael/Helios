// Package simulator drives one simulated vehicle: it moves a position forward
// tick by tick according to a behaviour profile, and streams the result to
// ingest over the TelemetryIngest gRPC service.
package simulator

import (
	"context"
	"math"
	"math/rand"
	"time"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
	"github.com/helios/trackline/apps/device-sim/internal/profiles"
)

type Vehicle struct {
	DeviceID string
	Profile  profiles.Profile

	lat, lon   float64
	speed      float64
	heading    float64
	ignitionOn bool
	odometerKm float64
	rng        *rand.Rand
}

func NewVehicle(deviceID string, p profiles.Profile, startLat, startLon float64, seed int64) *Vehicle {
	return &Vehicle{
		DeviceID: deviceID,
		Profile:  p,
		lat:      startLat,
		lon:      startLon,
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
		dLat := distanceKm / 111.0 * math.Cos(toRadians(v.heading))
		dLon := distanceKm / (111.0 * math.Cos(toRadians(v.lat))) * math.Sin(toRadians(v.heading))
		v.lat += dLat
		v.lon += dLon
	}

	noisyLat := v.lat + (v.rng.Float64()-0.5)*2*metresToDegrees(p.GPSNoiseM)
	noisyLon := v.lon + (v.rng.Float64()-0.5)*2*metresToDegrees(p.GPSNoiseM)

	return &tracklinev1.TelemetryPoint{
		DeviceId:      v.DeviceID,
		RecordedAt:    time.Now().UnixMilli(),
		Position:      &tracklinev1.GeoPoint{Latitude: noisyLat, Longitude: noisyLon},
		SpeedKph:      v.speed,
		HeadingDeg:    v.heading,
		AccuracyM:     p.GPSNoiseM,
		OdometerKm:    v.odometerKm,
		FuelLevelPct:  math.Max(0, 100-v.odometerKm*0.02),
		EngineTempC:   ifThenElse(v.ignitionOn, 85+v.rng.Float64()*10, 20),
		IgnitionOn:    v.ignitionOn,
	}
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

// StreamTo runs this vehicle's tick loop against a gRPC stream until ctx is
// cancelled. The caller owns the stream's lifecycle (connect/reconnect); this
// function only produces points.
func (v *Vehicle) StreamTo(ctx context.Context, send func(*tracklinev1.TelemetryPoint) error) error {
	interval := time.Duration(float64(time.Second) / v.Profile.EmissionHz)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			point := v.Tick(interval.Seconds())
			if err := send(point); err != nil {
				return err
			}
		}
	}
}
