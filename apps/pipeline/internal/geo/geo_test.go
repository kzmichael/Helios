package geo

import (
	"math"
	"testing"
)

// The seeded "Bengaluru Depot" rectangle.
var depot = Polygon{{77.5866, 12.9646}, {77.6026, 12.9646}, {77.6026, 12.9786}, {77.5866, 12.9786}, {77.5866, 12.9646}}

func TestContainsInsideAndOutside(t *testing.T) {
	if !depot.Contains(12.9716, 77.5946) {
		t.Fatal("depot centre should be inside")
	}
	if depot.Contains(12.9716, 77.5700) {
		t.Fatal("point west of the depot should be outside")
	}
	if depot.Contains(13.5, 77.5946) {
		t.Fatal("point far north should be outside")
	}
}

func TestContainsConcavePolygon(t *testing.T) {
	// An L-shape: the notch at (lon 2..4, lat 2..4) is outside.
	l := Polygon{{0, 0}, {4, 0}, {4, 2}, {2, 2}, {2, 4}, {0, 4}, {0, 0}}
	if !l.Contains(1, 1) || !l.Contains(3, 1) || !l.Contains(1, 3) {
		t.Fatal("L-shape arms should be inside")
	}
	if l.Contains(3, 3) {
		t.Fatal("the notch should be outside")
	}
}

func TestDegenerateRingContainsNothing(t *testing.T) {
	if (Polygon{{0, 0}, {1, 1}}).Contains(0.5, 0.5) {
		t.Fatal("a ring with fewer than 4 points has no interior")
	}
}

func TestHaversineKnownDistance(t *testing.T) {
	// One degree of latitude is about 111.2 km.
	d := Haversine(0, 0, 1, 0)
	if math.Abs(d-111195) > 300 {
		t.Fatalf("expected ~111195 m, got %f", d)
	}
	if Haversine(12.9, 77.5, 12.9, 77.5) != 0 {
		t.Fatal("identical points are zero metres apart")
	}
}
