// Package roles holds the consumer groups the pipeline binary can host. Each
// role is an independent Kafka consumer group (see apps/_contracts/kafka-topics.md
// §4), so it can be scaled, restarted, or moved to its own deployment alone.
package roles

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	"github.com/helios/trackline/apps/pipeline/internal/config"
	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
	"github.com/helios/trackline/apps/pipeline/internal/registry"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

type Deps struct {
	Cfg   config.Config
	Pool  *pgxpool.Pool
	Redis *redis.Client
	Reg   *registry.Registry
	Prod  *kafkax.Producer
	Log   *slog.Logger
}

// consume runs a consumer group for a role.
func (d Deps) consume(ctx context.Context, role string, topics []string, start int64, dlq string, h kafkax.Handler) error {
	return kafkax.Run(ctx, kafkax.Options{
		Brokers:     d.Cfg.Brokers,
		GroupID:     d.Cfg.Group(role),
		Topics:      topics,
		StartOffset: start,
		DLQTopic:    dlq,
		Producer:    d.Prod,
	}, h, d.Log.With("role", role))
}

// decodeTelemetry unmarshals a telemetry.raw message.
func decodeTelemetry(m kafkax.Message) (*tracklinev1.TelemetryReceived, error) {
	var ev tracklinev1.TelemetryReceived
	if err := proto.Unmarshal(m.Value, &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// telemetryTimes returns the device-recorded and ingest times of a point.
func telemetryTimes(ev *tracklinev1.TelemetryReceived) (recorded, ingested time.Time) {
	recorded = time.UnixMilli(ev.GetPoint().GetRecordedAt())
	ingested = time.UnixMilli(ev.GetIngestedAt())
	if ev.GetIngestedAt() == 0 {
		ingested = time.Now()
	}
	return recorded, ingested
}

// RegistrySync keeps the shared Redis device registry (read by ingest) fresh.
// Run exactly one replica of this role per environment.
func RegistrySync(ctx context.Context, d Deps) error {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		if err := d.Reg.Refresh(ctx); err != nil {
			d.Log.Error("registry refresh failed", "role", "registry-sync", "error", err)
		} else if err := d.Reg.PublishToRedis(ctx, d.Redis); err != nil {
			d.Log.Error("registry publish failed", "role", "registry-sync", "error", err)
		} else {
			d.Log.Info("registry published", "role", "registry-sync", "devices", len(d.Reg.Devices()))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
