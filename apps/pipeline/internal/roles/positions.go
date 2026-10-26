package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
	"github.com/helios/trackline/apps/pipeline/internal/registry"
)

// positionFrame is what the live map receives; it matches the Position schema in
// apps/_contracts/openapi.yaml.
type positionFrame struct {
	VehicleID    string  `json:"vehicleId"`
	Registration string  `json:"registration"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	SpeedKph     float64 `json:"speedKph"`
	HeadingDeg   float64 `json:"headingDeg"`
	IgnitionOn   bool    `json:"ignitionOn"`
	RecordedAt   string  `json:"recordedAt"`
	Stale        bool    `json:"stale"`
}

// PositionsChannel is the Redis pub/sub channel carrying one tenant's live positions.
func PositionsChannel(tenantID string) string { return "positions:" + tenantID }

type latest struct {
	dev        registry.Device
	recordedAt time.Time
	lastSeen   time.Time
	frame      positionFrame
	lat, lon   float64
	speed, hdg float64
	ignition   bool
}

// PositionUpdater is the position-updater consumer group: it maintains
// vehicle_position_latest (last write wins by recorded_at) and fans each new
// position out to Redis for the WebSocket map. Unknown devices are dead-lettered
// immediately — retrying cannot make an unprovisioned device known.
func PositionUpdater(ctx context.Context, d Deps) error {
	return d.consume(ctx, "position-updater", []string{kafkax.TopicTelemetryRaw}, kafka.LastOffset, kafkax.TopicTelemetryDLQ,
		func(ctx context.Context, msgs []kafkax.Message) error {
			newest := map[string]latest{}

			for _, m := range msgs {
				ev, err := decodeTelemetry(m)
				if err != nil {
					d.deadLetter(ctx, m, "schema_mismatch")
					continue
				}
				dev, ok := d.Reg.Lookup(ctx, ev.GetDeviceId())
				if !ok {
					d.deadLetter(ctx, m, "unknown_device")
					continue
				}
				recorded, ingested := telemetryTimes(ev)
				recorded = EffectiveTime(recorded, ingested)

				if cur, ok := newest[dev.VehicleID]; ok && !recorded.After(cur.recordedAt) {
					continue
				}
				p := ev.GetPoint()
				newest[dev.VehicleID] = latest{
					dev: dev, recordedAt: recorded, lastSeen: ingested,
					lat: p.GetPosition().GetLatitude(), lon: p.GetPosition().GetLongitude(),
					speed: p.GetSpeedKph(), hdg: p.GetHeadingDeg(), ignition: p.GetIgnitionOn(),
				}
			}
			if len(newest) == 0 {
				return nil
			}
			return d.writePositions(ctx, newest)
		})
}

func (d Deps) writePositions(ctx context.Context, newest map[string]latest) error {
	n := len(newest)
	vids, tids := make([]string, 0, n), make([]string, 0, n)
	lats, lons, speeds, hdgs := make([]float64, 0, n), make([]float64, 0, n), make([]float64, 0, n), make([]float64, 0, n)
	ign := make([]bool, 0, n)
	at, seen := make([]time.Time, 0, n), make([]time.Time, 0, n)
	devs := make([]string, 0, n)

	for vid, l := range newest {
		vids = append(vids, vid)
		tids = append(tids, l.dev.TenantID)
		devs = append(devs, l.dev.DeviceID)
		lats, lons = append(lats, l.lat), append(lons, l.lon)
		speeds, hdgs = append(speeds, l.speed), append(hdgs, l.hdg)
		ign = append(ign, l.ignition)
		at, seen = append(at, l.recordedAt), append(seen, l.lastSeen)
	}

	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO vehicle_position_latest (vehicle_id, tenant_id, lat, lon, speed_kph, heading_deg, ignition_on, recorded_at, updated_at)
		SELECT v::uuid, t::uuid, la, lo, sp, hd, ig, ts, now()
		FROM unnest($1::text[], $2::text[], $3::float8[], $4::float8[], $5::float8[], $6::float8[], $7::bool[], $8::timestamptz[])
		     AS x(v, t, la, lo, sp, hd, ig, ts)
		ON CONFLICT (vehicle_id) DO UPDATE
		   SET lat = EXCLUDED.lat, lon = EXCLUDED.lon, speed_kph = EXCLUDED.speed_kph,
		       heading_deg = EXCLUDED.heading_deg, ignition_on = EXCLUDED.ignition_on,
		       recorded_at = EXCLUDED.recorded_at, updated_at = now()
		 WHERE vehicle_position_latest.recorded_at < EXCLUDED.recorded_at`,
		vids, tids, lats, lons, speeds, hdgs, ign, at); err != nil {
		return fmt.Errorf("upsert positions: %w", err)
	}

	if _, err := d.Pool.Exec(ctx, `
		UPDATE device d SET last_seen_at = x.ts, status = 'active', updated_at = now()
		FROM unnest($1::text[], $2::timestamptz[]) AS x(id, ts)
		WHERE d.id = x.id::uuid AND (d.last_seen_at IS NULL OR d.last_seen_at < x.ts)`,
		devs, seen); err != nil {
		return fmt.Errorf("touch devices: %w", err)
	}

	pipe := d.Redis.Pipeline()
	for vid, l := range newest {
		b, _ := json.Marshal(positionFrame{
			VehicleID: vid, Registration: l.dev.Registration, Lat: l.lat, Lon: l.lon, SpeedKph: l.speed,
			HeadingDeg: l.hdg, IgnitionOn: l.ignition, RecordedAt: l.recordedAt.UTC().Format(time.RFC3339Nano),
		})
		pipe.Publish(ctx, PositionsChannel(l.dev.TenantID), b)
	}
	// The map still works from the database if Redis is down; do not fail the batch for it.
	if _, err := pipe.Exec(ctx); err != nil {
		d.Log.Warn("position fan-out failed", "role", "position-updater", "error", err)
	}
	return nil
}

func (d Deps) deadLetter(ctx context.Context, m kafkax.Message, reason string) {
	if err := d.Prod.DeadLetter(ctx, kafkax.TopicTelemetryDLQ, m, reason); err != nil {
		d.Log.Error("dead-letter failed", "reason", reason, "error", err)
	}
}
