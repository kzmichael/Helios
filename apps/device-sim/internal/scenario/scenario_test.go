package scenario

import (
	"testing"
	"time"
)

func TestEveryScenarioResolvesAProvisionedDevice(t *testing.T) {
	for _, name := range Names() {
		s, err := Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Identity(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestUnknownScenarioListsChoices(t *testing.T) {
	if _, err := Get("nope"); err == nil {
		t.Fatal("expected an error for an unknown scenario")
	}
}

func TestPointsAdvanceAlongHeading(t *testing.T) {
	s, _ := Get("port-entry")
	pts := s.Points("d", time.Now())
	if len(pts) != int(s.Duration/time.Second) {
		t.Fatalf("expected one point per second, got %d", len(pts))
	}
	first, last := pts[0].GetPosition(), pts[len(pts)-1].GetPosition()
	if last.GetLongitude() <= first.GetLongitude() {
		t.Fatal("eastbound drive should increase longitude")
	}
	if pts[1].GetRecordedAt()-pts[0].GetRecordedAt() != 1000 {
		t.Fatal("points should be one second apart")
	}
}

// The port-entry drive must actually cross the seeded Restricted Port Area
// (lon 72.8320-72.8520 at lat ~18.948), or the scenario proves nothing.
func TestPortEntryCrossesTheExclusionZone(t *testing.T) {
	s, _ := Get("port-entry")
	inside := 0
	for _, p := range s.Points("d", time.Now()) {
		lon, lat := p.GetPosition().GetLongitude(), p.GetPosition().GetLatitude()
		if lon > 72.8320 && lon < 72.8520 && lat > 18.9380 && lat < 18.9580 {
			inside++
		}
	}
	if inside == 0 {
		t.Fatal("scenario never enters the Restricted Port Area")
	}
}
