// Package grpcserver implements the TelemetryIngest service defined in
// apps/_proto/trackline/v1/telemetry.proto. This is the hot path: every field
// touched here is multiplied by device count times emission rate.
package grpcserver

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
	"github.com/helios/trackline/apps/ingest/internal/kafka"
	"github.com/helios/trackline/apps/ingest/internal/validate"
)

// dedupeWindow bounds how long a batch_id is remembered for PublishBatch
// deduplication. A real deployment backs this with Redis; Phase 0 uses an
// in-process map, which is intentionally not durable across restarts.
const dedupeWindow = 10 * time.Minute

type Server struct {
	tracklinev1.UnimplementedTelemetryIngestServer

	producer *kafkaproducer.Producer
	logger   *slog.Logger

	seenBatches map[string]time.Time
}

func New(producer *kafkaproducer.Producer, logger *slog.Logger) *Server {
	return &Server{
		producer:    producer,
		logger:      logger,
		seenBatches: make(map[string]time.Time),
	}
}

func (s *Server) Health(ctx context.Context, _ *tracklinev1.HealthRequest) (*tracklinev1.HealthResponse, error) {
	return &tracklinev1.HealthResponse{Ready: true}, nil
}

// StreamTelemetry is the preferred transport for connected devices — see
// telemetry.proto. It never returns a hard error for pacing; it sets
// retry_after_ms and lets the caller decide, per limits.md §5's backpressure
// design. Only devices that ignore that repeatedly get throttled outright,
// which is a decision made above this layer (rate limiting, not here).
func (s *Server) StreamTelemetry(stream tracklinev1.TelemetryIngest_StreamTelemetryServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		resp := &tracklinev1.StreamTelemetryResponse{Errors: map[uint32]string{}}

		for i, point := range req.GetPoints() {
			if err := validate.Point(point); err != nil {
				resp.Rejected++
				resp.Errors[uint32(i)] = err.Error()
				continue
			}

			if err := s.publish(stream.Context(), point.GetDeviceId(), point); err != nil {
				s.logger.Error("publish failed", "error", err, "device_id", point.GetDeviceId())
				resp.Rejected++
				resp.Errors[uint32(i)] = "internal: publish failed"
				continue
			}
			resp.Accepted++
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// PublishBatch is the unary fallback for devices that cannot hold a stream
// open. batch_id makes redelivery of the same batch a no-op rather than a
// duplicate — see kafka-topics.md §3 on at-least-once delivery.
func (s *Server) PublishBatch(ctx context.Context, req *tracklinev1.PublishBatchRequest) (*tracklinev1.PublishBatchResponse, error) {
	s.evictExpiredBatches()

	if seenAt, ok := s.seenBatches[req.GetBatchId()]; ok {
		s.logger.Info("duplicate batch ignored", "batch_id", req.GetBatchId(), "first_seen", seenAt)
		return &tracklinev1.PublishBatchResponse{Duplicate: true}, nil
	}

	accepted := uint32(0)
	for _, point := range req.GetPoints() {
		if err := validate.Point(point); err != nil {
			continue
		}
		if err := s.publish(ctx, req.GetDeviceId(), point); err != nil {
			s.logger.Error("publish failed", "error", err, "device_id", req.GetDeviceId())
			continue
		}
		accepted++
	}

	s.seenBatches[req.GetBatchId()] = time.Now()
	return &tracklinev1.PublishBatchResponse{Accepted: accepted}, nil
}

func (s *Server) publish(ctx context.Context, deviceID string, point *tracklinev1.TelemetryPoint) error {
	event := &tracklinev1.TelemetryReceived{
		DeviceId:    deviceID,
		Point:       point,
		IngestedAt:  time.Now().UnixMilli(),
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return err
	}
	// Keyed by device_id here; the vehicle/device mapping is resolved
	// downstream by position-updater, which is why the topic's documented key
	// (kafka-topics.md) is vehicle_id at the consumer's semantic level even
	// though ingest only knows the device at this point.
	return s.producer.Publish(ctx, deviceID, payload)
}

func (s *Server) evictExpiredBatches() {
	cutoff := time.Now().Add(-dedupeWindow)
	for id, seenAt := range s.seenBatches {
		if seenAt.Before(cutoff) {
			delete(s.seenBatches, id)
		}
	}
}
