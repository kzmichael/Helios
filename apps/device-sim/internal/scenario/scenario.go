// Package scenario scripts a single device along a fixed path so a specific alert
// can be triggered on demand, instead of waiting for a random walk to cross a
// geofence or break a speed limit. Each scenario targets a seeded alert rule.
package scenario

import (
	"fmt"
	"sort"
	"time"

	"github.com/helios/trackline/apps/device-sim/internal/identity"
	"github.com/helios/trackline/apps/device-sim/internal/simulator"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// Script is a straight-line drive: start, heading, speed, duration, then optional silence.
type Script struct {
	Name        string
	Description string

	Fleet, Vehicle int
	StartLat       float64
	StartLon       float64
	HeadingDeg     float64
	SpeedKph       float64
	Duration       time.Duration

	// SilentFor keeps the process alive without emitting, so a device-offline rule
	// can observe the silence. Zero exits as soon as the drive ends.
	SilentFor time.Duration
}

var scripts = map[string]Script{
	// Meridian (pro): drives east through the "Restricted Port Area" exclusion
	// zone -> "Entered port area" (critical) over email and webhook.
	"port-entry": {
		Description: "Meridian vehicle enters the Restricted Port Area",
		Fleet:       2, Vehicle: 1,
		StartLat: 18.9480, StartLon: 72.8210, HeadingDeg: 90, SpeedKph: 50,
		Duration: 4 * time.Minute,
	},
	// Meridian (pro): starts inside the Mumbai Hub and drives out -> "Left the hub" (info).
	"hub-exit": {
		Description: "Meridian vehicle leaves the Mumbai Hub",
		Fleet:       2, Vehicle: 2,
		StartLat: 19.0760, StartLon: 72.8777, HeadingDeg: 90, SpeedKph: 50,
		Duration: 3 * time.Minute,
	},
	// Atlas (enterprise): well over its limit for longer than the rule's minimum
	// duration -> "Speeding" (warning).
	"speeding": {
		Description: "Atlas vehicle speeds for 90 seconds",
		Fleet:       4, Vehicle: 1,
		StartLat: 28.5500, StartLon: 77.1000, HeadingDeg: 45, SpeedKph: 120,
		Duration: 90 * time.Second,
	},
	// Atlas (enterprise): reports briefly then goes silent -> "Device offline"
	// (critical), which also reaches the SMS channel.
	"offline": {
		Description: "Atlas vehicle reports for 20 seconds then goes silent",
		Fleet:       4, Vehicle: 2,
		StartLat: 28.6139, StartLon: 77.2090, HeadingDeg: 0, SpeedKph: 40,
		Duration:  20 * time.Second,
		SilentFor: 8 * time.Minute,
	},
}

// Names lists the available scenarios.
func Names() []string {
	names := make([]string, 0, len(scripts))
	for n := range scripts {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Get resolves a scenario by name.
func Get(name string) (Script, error) {
	s, ok := scripts[name]
	if !ok {
		return Script{}, fmt.Errorf("unknown scenario %q (available: %v)", name, Names())
	}
	s.Name = name
	return s, nil
}

// Identity is the provisioned device that plays this scenario.
func (s Script) Identity() (identity.Identity, error) {
	id, ok := identity.Lookup(s.Fleet, s.Vehicle)
	if !ok {
		return identity.Identity{}, fmt.Errorf("scenario %s: no provisioned device for fleet %d vehicle %d", s.Name, s.Fleet, s.Vehicle)
	}
	return id, nil
}

// Points renders the drive as one point per second starting at start.
func (s Script) Points(deviceID string, start time.Time) []*tracklinev1.TelemetryPoint {
	n := int(s.Duration / time.Second)
	points := make([]*tracklinev1.TelemetryPoint, 0, n)
	lat, lon := s.StartLat, s.StartLon
	odometer := 0.0
	for i := 0; i < n; i++ {
		points = append(points, &tracklinev1.TelemetryPoint{
			DeviceId:    deviceID,
			RecordedAt:  start.Add(time.Duration(i) * time.Second).UnixMilli(),
			Position:    &tracklinev1.GeoPoint{Latitude: lat, Longitude: lon},
			SpeedKph:    s.SpeedKph,
			HeadingDeg:  s.HeadingDeg,
			AccuracyM:   5,
			OdometerKm:  odometer,
			IgnitionOn:  true,
			EngineTempC: 90,
		})
		step := s.SpeedKph / 3600.0
		odometer += step
		lat, lon = simulator.Advance(lat, lon, s.HeadingDeg, step)
	}
	return points
}
