// Package geo holds the geometry the pipeline needs: point-in-polygon for
// geofences and a quick great-circle distance.
package geo

import "math"

// Polygon is a closed ring of [lon, lat] vertices, GeoJSON order.
type Polygon [][2]float64

// Contains reports whether (lat, lon) lies inside p using ray casting. Points on
// the boundary may fall either way; GPS noise makes that distinction meaningless.
func (p Polygon) Contains(lat, lon float64) bool {
	n := len(p)
	if n < 4 {
		return false
	}
	inside := false
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := p[i][0], p[i][1]
		xj, yj := p[j][0], p[j][1]
		if (yi > lat) != (yj > lat) && lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

// Haversine returns the distance in metres between two WGS84 points.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
