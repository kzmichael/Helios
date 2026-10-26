package roles

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
)

type telemetryRow struct {
	tenant, vehicle, device                       string
	recorded, ingested                            time.Time
	lat, lon, speed, heading                      float64
	altitude, accuracy, odometer, fuel, engineTmp float64
	ignition                                      bool
}

// TelemetryArchiver is the telemetry-archiver consumer group: it batch-appends
// every accepted point to the telemetry table, which backs /vehicles/{id}/history.
// It reads from the earliest offset when new, because a gap in history is a gap in
// a customer's track.
func TelemetryArchiver(ctx context.Context, d Deps) error {
	return d.consume(ctx, "telemetry-archiver", []string{kafkax.TopicTelemetryRaw}, kafka.FirstOffset, kafkax.TopicTelemetryDLQ,
		func(ctx context.Context, msgs []kafkax.Message) error {
			rows := make([]telemetryRow, 0, len(msgs))
			for _, m := range msgs {
				ev, err := decodeTelemetry(m)
				if err != nil {
					continue // already dead-lettered by position-updater
				}
				dev, ok := d.Reg.Lookup(ctx, ev.GetDeviceId())
				if !ok {
					continue // unprovisioned device: dead-lettered by position-updater
				}
				p := ev.GetPoint()
				recorded, ingested := telemetryTimes(ev)
				rows = append(rows, telemetryRow{
					tenant: dev.TenantID, vehicle: dev.VehicleID, device: dev.DeviceID,
					recorded: recorded, ingested: ingested,
					lat: p.GetPosition().GetLatitude(), lon: p.GetPosition().GetLongitude(),
					speed: clamp(p.GetSpeedKph(), 0, 9999), heading: clamp(p.GetHeadingDeg(), 0, 999),
					altitude: clamp(p.GetAltitudeM(), -99999, 99999), accuracy: clamp(p.GetAccuracyM(), 0, 9999),
					odometer: clamp(p.GetOdometerKm(), 0, 99999999), fuel: clamp(p.GetFuelLevelPct(), 0, 100),
					engineTmp: clamp(p.GetEngineTempC(), -99, 999), ignition: p.GetIgnitionOn(),
				})
			}
			return insertTelemetry(ctx, d, rows)
		})
}

// clamp keeps a value inside its column's numeric range so one odd reading cannot
// fail a whole batch; NaN becomes zero.
func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(lo, math.Min(hi, v))
}

func insertTelemetry(ctx context.Context, d Deps, rows []telemetryRow) error {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	tenants, vehicles, devices := make([]string, 0, n), make([]string, 0, n), make([]string, 0, n)
	rec, ing := make([]time.Time, 0, n), make([]time.Time, 0, n)
	f := make([][]float64, 9)
	for i := range f {
		f[i] = make([]float64, 0, n)
	}
	ign := make([]bool, 0, n)

	for _, r := range rows {
		tenants, vehicles, devices = append(tenants, r.tenant), append(vehicles, r.vehicle), append(devices, r.device)
		rec, ing = append(rec, r.recorded), append(ing, r.ingested)
		for i, v := range []float64{r.lat, r.lon, r.speed, r.heading, r.altitude, r.accuracy, r.odometer, r.fuel, r.engineTmp} {
			f[i] = append(f[i], v)
		}
		ign = append(ign, r.ignition)
	}

	_, err := d.Pool.Exec(ctx, `
		INSERT INTO telemetry (tenant_id, vehicle_id, device_id, recorded_at, ingested_at, lat, lon, speed_kph,
		                       heading_deg, altitude_m, accuracy_m, odometer_km, fuel_level_pct, engine_temp_c, ignition_on)
		SELECT t::uuid, v::uuid, d::uuid, r, i, la, lo, sp, hd, al, ac, od, fu, et, ig
		FROM unnest($1::text[], $2::text[], $3::text[], $4::timestamptz[], $5::timestamptz[],
		            $6::float8[], $7::float8[], $8::float8[], $9::float8[], $10::float8[],
		            $11::float8[], $12::float8[], $13::float8[], $14::float8[], $15::bool[])
		     AS x(t, v, d, r, i, la, lo, sp, hd, al, ac, od, fu, et, ig)`,
		tenants, vehicles, devices, rec, ing, f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7], f[8], ign)
	if err != nil {
		return fmt.Errorf("insert telemetry: %w", err)
	}
	return nil
}
