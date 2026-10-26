package identity

import "testing"

func TestBuildCoversProvisionedThenUnregistered(t *testing.T) {
	ids := Build(520, 12.97, 77.59, true)
	if len(ids) != 520 {
		t.Fatalf("expected 520 identities, got %d", len(ids))
	}
	for i := 0; i < 500; i++ {
		if !ids[i].Registered {
			t.Fatalf("identity %d should be provisioned", i)
		}
	}
	for i := 500; i < 520; i++ {
		if ids[i].Registered {
			t.Fatalf("identity %d should be unregistered", i)
		}
	}
}

func TestIdentitiesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range Build(600, 0, 0, true) {
		if seen[id.DeviceID] {
			t.Fatalf("duplicate device id %s", id.DeviceID)
		}
		seen[id.DeviceID] = true
	}
}

func TestSeedIdentitiesMatchSeedScheme(t *testing.T) {
	id, ok := Lookup(2, 1)
	if !ok {
		t.Fatal("fleet 2 vehicle 1 should exist")
	}
	if id.VehicleID != "c0000000-0000-4000-8000-000000020001" {
		t.Fatalf("unexpected vehicle id %s", id.VehicleID)
	}
	if id.DeviceID != "de000000-0000-4000-8000-000000020001" {
		t.Fatalf("unexpected device id %s", id.DeviceID)
	}
	if id.Token != "dev-token-"+id.VehicleID {
		t.Fatalf("unexpected token %s", id.Token)
	}
}

func TestLookupRejectsOutOfRange(t *testing.T) {
	if _, ok := Lookup(1, 6); ok {
		t.Fatal("fleet 1 only has 5 vehicles")
	}
	if _, ok := Lookup(99, 1); ok {
		t.Fatal("fleet 99 does not exist")
	}
}

func TestOriginPlacementOverridesDepots(t *testing.T) {
	ids := Build(10, 1.5, 2.5, false)
	for _, id := range ids {
		if id.Lat != 1.5 || id.Lon != 2.5 {
			t.Fatalf("expected origin placement, got %f,%f", id.Lat, id.Lon)
		}
	}
}
