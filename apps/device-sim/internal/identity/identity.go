// Package identity derives simulated device identities that line up with the
// provisioned devices in apps/_db, so telemetry resolves to real vehicles.
//
// The seed and the opt-in load fleet (apps/_db/load/load-fleet.sql) build ids
// from the same (fleet ordinal, vehicle number) scheme used here.
package identity

import "fmt"

// Fleet describes one provisioned fleet: its ordinal in the id scheme, how many
// vehicles it holds, and the depot the vehicles start around.
type Fleet struct {
	Ord      int
	Vehicles int
	Lat, Lon float64
}

// Seeded mirrors apps/_db/seed (150 vehicles). Load mirrors the opt-in load
// fleet (350 vehicles) so a full run reaches 500 provisioned devices.
var (
	Seeded = []Fleet{
		{1, 5, 12.9716, 77.5946},
		{2, 15, 19.0760, 72.8777},
		{3, 10, 19.1136, 72.8697},
		{4, 40, 28.6139, 77.2090},
		{5, 30, 18.5204, 73.8567},
		{6, 30, 12.9716, 77.5946},
		{7, 20, 17.3850, 78.4867},
	}
	Load = []Fleet{
		{8, 45, 12.9716, 77.5946},
		{9, 175, 19.0760, 72.8777},
		{10, 130, 28.6139, 77.2090},
	}
)

// Identity is one simulated device. Registered is false for devices beyond the
// provisioned set; those still emit telemetry but resolve to no vehicle.
type Identity struct {
	DeviceID   string
	VehicleID  string
	Token      string
	Lat, Lon   float64
	Registered bool
}

func suffix(ord, n int) string { return fmt.Sprintf("%012d", ord*10000+n) }

// VehicleID is the provisioned vehicle uuid for fleet ordinal ord, vehicle n.
func VehicleID(ord, n int) string { return "c0000000-0000-4000-8000-" + suffix(ord, n) }

// DeviceID is the provisioned device uuid paired with that vehicle.
func DeviceID(ord, n int) string { return "de000000-0000-4000-8000-" + suffix(ord, n) }

// Token is the bearer token the seed provisions for a vehicle's device. Only its
// SHA-256 is stored server-side.
func Token(vehicleID string) string { return "dev-token-" + vehicleID }

// Lookup returns the identity for fleet ordinal ord and vehicle number n, or false
// if no such fleet is part of the provisioned set.
func Lookup(ord, n int) (Identity, bool) {
	for _, f := range append(append([]Fleet{}, Seeded...), Load...) {
		if f.Ord == ord && n >= 1 && n <= f.Vehicles {
			vid := VehicleID(ord, n)
			return Identity{DeviceID: DeviceID(ord, n), VehicleID: vid, Token: Token(vid), Lat: f.Lat, Lon: f.Lon, Registered: true}, true
		}
	}
	return Identity{}, false
}

// Build returns count identities: provisioned devices first (seeded fleets, then
// the load fleets), then unregistered ones scattered around (originLat, originLon).
// When fleetPlacement is false every device starts at the origin.
func Build(count int, originLat, originLon float64, fleetPlacement bool) []Identity {
	out := make([]Identity, 0, count)
	for _, f := range append(append([]Fleet{}, Seeded...), Load...) {
		for n := 1; n <= f.Vehicles && len(out) < count; n++ {
			id, _ := Lookup(f.Ord, n)
			if !fleetPlacement {
				id.Lat, id.Lon = originLat, originLon
			}
			out = append(out, id)
		}
	}
	for i := len(out); i < count; i++ {
		out = append(out, Identity{
			DeviceID: fmt.Sprintf("sim-device-%05d", i),
			Lat:      originLat,
			Lon:      originLon,
		})
	}
	return out
}
