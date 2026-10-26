// Package grpcserver implements the TelemetryIngest service defined in
// apps/_proto/trackline/v1/telemetry.proto. This is the hot path: every field
// touched here is multiplied by device count times emission rate.
package grpcserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/helios/trackline/apps/ingest/internal/backpressure"
	kafkaproducer "github.com/helios/trackline/apps/ingest/internal/kafka"
	"github.com/helios/trackline/apps/ingest/internal/ratelimit"
	"github.com/helios/trackline/apps/ingest/internal/registry"
	"github.com/helios/trackline/apps/ingest/internal/validate"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// maxBatchPoints is the PublishBatch cap from limits.md §3.
const maxBatchPoints = 1000

// retryAfterShedMs is what a shed or rate-limited device is asked to wait.
const retryAfterShedMs = 1000

// Options turns on the capabilities that need the shared device registry. With the
// zero value ingest behaves as a plain, unauthenticated sink.
type Options struct {
	// Registry resolves device ids to tenant, vehicle and tier. Nil disables auth,
	// per-device rate limits and tier-aware shedding.
	Registry *registry.Cache

	// AuthRequired rejects points from unknown devices or with a bad device token.
	AuthRequired bool

	// Shedder, when set, applies limits.md §6 load shedding by tier.
	Shedder *backpressure.Shedder

	// Dedupe remembers PublishBatch ids. Nil uses an in-process map.
	Dedupe Deduper
}

type Server struct {
	tracklinev1.UnimplementedTelemetryIngestServer

	producer *kafkaproducer.Producer
	logger   *slog.Logger
	opts     Options

	limiters sync.Map // device id -> *ratelimit.Bucket
}

func New(producer *kafkaproducer.Producer, logger *slog.Logger, opts Options) *Server {
	if opts.Dedupe == nil {
		opts.Dedupe = NewMemoryDeduper()
	}
	return &Server{producer: producer, logger: logger, opts: opts}
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
	token := bearerToken(stream.Context())
	admitted := map[string]admission{} // per-stream memo, so tokens are hashed once per device

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

			adm, ok := admitted[point.GetDeviceId()]
			if !ok {
				adm = s.admit(stream.Context(), token, point.GetDeviceId())
				admitted[point.GetDeviceId()] = adm
			}
			if adm.reason != "" {
				resp.Rejected++
				resp.Errors[uint32(i)] = adm.reason
				continue
			}

			if reason, retryMs := s.throttle(adm); reason != "" {
				resp.Rejected++
				resp.Errors[uint32(i)] = reason
				if retryMs > resp.RetryAfterMs {
					resp.RetryAfterMs = retryMs
				}
				continue
			}

			if err := s.publish(stream.Context(), adm, point); err != nil {
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
	if len(req.GetPoints()) > maxBatchPoints {
		return nil, status.Errorf(codes.InvalidArgument, "batch has %d points; the limit is %d", len(req.GetPoints()), maxBatchPoints)
	}

	if req.GetBatchId() != "" && s.opts.Dedupe.Seen(ctx, req.GetBatchId()) {
		s.logger.Info("duplicate batch ignored", "batch_id", req.GetBatchId())
		return &tracklinev1.PublishBatchResponse{Duplicate: true}, nil
	}

	token := bearerToken(ctx)
	admitted := map[string]admission{}
	accepted := uint32(0)
	for _, point := range req.GetPoints() {
		if point.GetDeviceId() == "" {
			point.DeviceId = req.GetDeviceId()
		}
		if err := validate.Point(point); err != nil {
			continue
		}
		adm, ok := admitted[point.GetDeviceId()]
		if !ok {
			adm = s.admit(ctx, token, point.GetDeviceId())
			admitted[point.GetDeviceId()] = adm
		}
		if adm.reason != "" {
			continue
		}
		if reason, _ := s.throttle(adm); reason != "" {
			continue
		}
		if err := s.publish(ctx, adm, point); err != nil {
			s.logger.Error("publish failed", "error", err, "device_id", point.GetDeviceId())
			continue
		}
		accepted++
	}

	// Remember the batch only once it has been handled, so a retry of a batch that
	// failed outright is processed rather than mistaken for a duplicate.
	if req.GetBatchId() != "" {
		s.opts.Dedupe.Mark(ctx, req.GetBatchId())
	}
	return &tracklinev1.PublishBatchResponse{Accepted: accepted}, nil
}

// admission is the outcome of resolving and authenticating one device.
type admission struct {
	deviceID string
	dev      registry.Device
	known    bool
	reason   string // non-empty: reject the device's points with this error
}

// admit resolves a device and, when required, checks its bearer token.
func (s *Server) admit(ctx context.Context, token, deviceID string) admission {
	adm := admission{deviceID: deviceID}
	if s.opts.Registry == nil {
		return adm
	}
	dev, ok := s.opts.Registry.Lookup(ctx, deviceID)
	adm.dev, adm.known = dev, ok

	switch {
	case !ok && s.opts.AuthRequired:
		adm.reason = "unauthenticated: unknown device"
	case ok && s.opts.AuthRequired && !dev.TokenMatches(token):
		adm.reason = "unauthenticated: bad device token"
	}
	return adm
}

// throttle applies the per-device rate limit (limits.md §3) and load shedding
// (§6). It returns a rejection reason and a retry hint, or "" to accept.
func (s *Server) throttle(adm admission) (string, uint32) {
	tier := "free"
	if adm.known {
		tier = adm.dev.Tier

		hz := adm.dev.MaxHz
		if hz <= 0 {
			hz = 1
		}
		v, _ := s.limiters.LoadOrStore(adm.deviceID, ratelimit.NewBucket(hz, 5))
		if !v.(*ratelimit.Bucket).Allow(time.Now()) {
			return "rate limited", uint32(1000 / hz)
		}
	}

	if s.opts.Shedder != nil {
		switch s.opts.Shedder.Decide(tier) {
		case backpressure.Reject:
			return "shed: overloaded", retryAfterShedMs
		case backpressure.Sample:
			return "shed: sampled", retryAfterShedMs
		}
	}
	return "", 0
}

// bearerToken reads the device token from the gRPC authorization metadata.
func bearerToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, v := range md.Get("authorization") {
		if t, found := strings.CutPrefix(v, "Bearer "); found {
			return t
		}
	}
	return ""
}

func newEventID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// publish wraps the point in a TelemetryReceived event and writes it to Kafka. A
// provisioned device is keyed by vehicle_id (kafka-topics.md §2); an unknown one
// falls back to its device id.
func (s *Server) publish(ctx context.Context, adm admission, point *tracklinev1.TelemetryPoint) error {
	event := &tracklinev1.TelemetryReceived{
		Meta: &tracklinev1.EventMeta{
			EventId:        newEventID(),
			TenantId:       adm.dev.Tenant,
			Source:         "ingest",
			OccurredAt:     point.GetRecordedAt(),
			IdempotencyKey: fmt.Sprintf("%s:%d", adm.deviceID, point.GetRecordedAt()),
		},
		VehicleId:  adm.dev.Vehicle,
		DeviceId:   adm.deviceID,
		Point:      point,
		IngestedAt: time.Now().UnixMilli(),
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return err
	}

	key := adm.deviceID
	if adm.known {
		key = adm.dev.Vehicle
	}

	if s.opts.Shedder == nil {
		return s.producer.Publish(ctx, key, payload)
	}
	done := s.opts.Shedder.Begin()
	err = s.producer.Publish(ctx, key, payload)
	done(err)
	return err
}
