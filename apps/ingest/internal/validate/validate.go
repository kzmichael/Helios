// Package validate applies the range checks a TelemetryPoint must pass before it
// is trusted enough to publish onto Kafka. Rejects are counted per point, not per
// batch — one bad point must not sink an otherwise-good batch (telemetry.proto's
// StreamTelemetryResponse.errors is keyed by position for exactly this reason).
package validate

import (
	"fmt"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

func Point(p *tracklinev1.TelemetryPoint) error {
	if p.GetDeviceId() == "" {
		return fmt.Errorf("device_id is required")
	}
	lat := p.GetPosition().GetLatitude()
	lon := p.GetPosition().GetLongitude()
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude %f out of range", lat)
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("longitude %f out of range", lon)
	}
	if p.GetSpeedKph() < 0 || p.GetSpeedKph() > 300 {
		return fmt.Errorf("speed_kph %f implausible", p.GetSpeedKph())
	}
	if p.GetHeadingDeg() < 0 || p.GetHeadingDeg() >= 360 {
		return fmt.Errorf("heading_deg %f out of [0,360)", p.GetHeadingDeg())
	}
	if p.GetRecordedAt() <= 0 {
		return fmt.Errorf("recorded_at is required")
	}
	return nil
}
