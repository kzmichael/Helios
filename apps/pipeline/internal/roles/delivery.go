package roles

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// maxDeliveryAttempts mirrors the notifier's retry budget; a failure at or past it is final.
const maxDeliveryAttempts = 5

// DeliveryStatus maps a notifier outcome to a notification_delivery status.
func DeliveryStatus(success bool, attempt uint32) string {
	switch {
	case success:
		return "sent"
	case attempt >= maxDeliveryAttempts:
		return "dead"
	default:
		return "failed"
	}
}

// DeliveryRecorder is the delivery-recorder consumer group: it writes the
// notifier's outcomes back to notification_delivery so the platform can see what
// was actually sent.
func DeliveryRecorder(ctx context.Context, d Deps) error {
	return d.consume(ctx, "delivery-recorder", []string{kafkax.TopicNotificationsDone}, kafka.FirstOffset, "",
		func(ctx context.Context, msgs []kafkax.Message) error {
			for _, m := range msgs {
				var e tracklinev1.NotificationDelivered
				if err := proto.Unmarshal(m.Value, &e); err != nil {
					d.Log.Warn("skipping undecodable delivery", "role", "delivery-recorder", "error", err)
					continue
				}
				var errText any
				if e.GetError() != "" {
					errText = e.GetError()
				}
				var code any
				if e.GetStatusCode() != 0 {
					code = e.GetStatusCode()
				}
				if _, err := d.Pool.Exec(ctx, `
					UPDATE notification_delivery
					   SET status = $3, attempts = GREATEST(attempts, $4), status_code = $5, last_error = $6,
					       latency_ms = $7, last_attempt_at = now(), first_attempt_at = coalesce(first_attempt_at, now())
					 WHERE alert_id = $1::uuid AND channel_id = $2::uuid`,
					e.GetAlertId(), e.GetChannelId(), DeliveryStatus(e.GetSuccess(), e.GetAttempt()),
					int(e.GetAttempt()), code, errText, int(e.GetLatencyMs())); err != nil {
					return fmt.Errorf("record delivery: %w", err)
				}
			}
			return nil
		})
}

// UsageCollector is the billing-collector consumer group: it adds each usage delta
// to usage_daily. A repeated delivery is dropped by its idempotency key so
// at-least-once delivery cannot inflate an invoice.
func UsageCollector(ctx context.Context, d Deps) error {
	return d.consume(ctx, "usage-collector", []string{kafkax.TopicUsageRecorded}, kafka.FirstOffset, "",
		func(ctx context.Context, msgs []kafkax.Message) error {
			for _, m := range msgs {
				var e tracklinev1.UsageRecorded
				if err := proto.Unmarshal(m.Value, &e); err != nil {
					d.Log.Warn("skipping undecodable usage event", "role", "usage-collector", "error", err)
					continue
				}
				key := e.GetMeta().GetIdempotencyKey()
				if key != "" {
					first, err := d.Redis.SetNX(ctx, "usage:seen:"+key, 1, 48*time.Hour).Result()
					if err == nil && !first {
						continue
					}
				}
				if _, err := d.Pool.Exec(ctx, `
					INSERT INTO usage_daily (tenant_id, usage_date, telemetry_points, active_devices, api_requests, storage_bytes)
					VALUES ($1::uuid, $2::date, $3, $4, $5, $6)
					ON CONFLICT (tenant_id, usage_date) DO UPDATE
					   SET telemetry_points = usage_daily.telemetry_points + EXCLUDED.telemetry_points,
					       active_devices   = GREATEST(usage_daily.active_devices, EXCLUDED.active_devices),
					       api_requests     = usage_daily.api_requests + EXCLUDED.api_requests,
					       storage_bytes    = GREATEST(usage_daily.storage_bytes, EXCLUDED.storage_bytes),
					       computed_at      = now()`,
					e.GetMeta().GetTenantId(), e.GetUsageDate(), int64(e.GetTelemetryPoints()), int(e.GetActiveDevices()),
					int64(e.GetApiRequests()), int64(e.GetStorageBytes())); err != nil {
					return fmt.Errorf("record usage: %w", err)
				}
			}
			return nil
		})
}
