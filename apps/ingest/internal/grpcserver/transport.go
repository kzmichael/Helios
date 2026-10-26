package grpcserver

import (
	"context"

	"github.com/helios/trackline/apps/ingest/internal/validate"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// IngestBatch ingests points that arrived over a transport other than gRPC (MQTT).
// The transport has already authenticated the device — the broker's ACL only lets a
// device publish to its own topic — so no bearer token is checked here. Everything
// else (validation, the registry, per-device limits, shedding, de-duplication)
// applies exactly as it does for PublishBatch.
func (s *Server) IngestBatch(ctx context.Context, deviceID, batchID string, points []*tracklinev1.TelemetryPoint) (accepted uint32, duplicate bool) {
	if len(points) > maxBatchPoints {
		points = points[:maxBatchPoints]
	}
	if batchID != "" && s.opts.Dedupe.Seen(ctx, batchID) {
		return 0, true
	}

	adm := admission{deviceID: deviceID}
	if s.opts.Registry != nil {
		adm.dev, adm.known = s.opts.Registry.Lookup(ctx, deviceID)
		if !adm.known && s.opts.AuthRequired {
			return 0, false
		}
	}

	for _, p := range points {
		p.DeviceId = deviceID // the topic, not the payload, names the device
		if validate.Point(p) != nil {
			continue
		}
		if reason, _ := s.throttle(adm); reason != "" {
			continue
		}
		if err := s.publish(ctx, adm, p); err != nil {
			s.logger.Error("publish failed", "error", err, "device_id", deviceID)
			continue
		}
		accepted++
	}

	if batchID != "" {
		s.opts.Dedupe.Mark(ctx, batchID)
	}
	return accepted, false
}
