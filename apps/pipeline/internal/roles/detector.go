package roles

import (
	"context"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	"github.com/helios/trackline/apps/pipeline/internal/events"
	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
	"github.com/helios/trackline/apps/pipeline/internal/registry"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

const detectorSource = "pipeline/detector"

type usageAcc struct {
	points  uint64
	devices map[string]struct{}
}

// DetectorRole is the Phase 0 inline stream processor: it turns telemetry into
// geofence, violation and offline events and into per-tenant usage. It emits the
// same events and carries the same Kafka contract as the Phase 2 Flink job
// (apps/stream-proc); run one or the other, not both.
func DetectorRole(ctx context.Context, d Deps) error {
	det := NewDetector()
	var mu sync.Mutex
	usage := map[string]*usageAcc{}
	instance := events.NewID()
	var seq uint64

	_ = d.Reg.Refresh(ctx)
	det.Seed(d.Reg.Devices(), time.Now())

	publish := func(ctx context.Context, evs []Event) error {
		msgs := make([]kafka.Message, 0, len(evs))
		for _, e := range evs {
			m, err := encodeEvent(e)
			if err != nil {
				return err
			}
			msgs = append(msgs, m)
		}
		return d.Prod.PublishAll(ctx, msgs)
	}

	go func() { // offline sweep and usage flush
		sweep := time.NewTicker(15 * time.Second)
		flush := time.NewTicker(30 * time.Second)
		defer sweep.Stop()
		defer flush.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-sweep.C:
				mu.Lock()
				evs := det.Sweep(time.Now(), func(vid string) (registry.Device, bool) { return d.Reg.Vehicle(ctx, vid) })
				mu.Unlock()
				if err := publish(ctx, evs); err != nil {
					d.Log.Error("offline publish failed", "role", "detector", "error", err)
				}
			case <-flush.C:
				mu.Lock()
				snapshot := usage
				usage = map[string]*usageAcc{}
				mu.Unlock()
				seq++
				if err := publishUsage(ctx, d, snapshot, instance, seq); err != nil {
					d.Log.Error("usage publish failed, will retry next window", "role", "detector", "error", err)
					mu.Lock()
					for t, a := range snapshot { // keep the counts rather than lose revenue data
						cur := usage[t]
						if cur == nil {
							usage[t] = a
							continue
						}
						cur.points += a.points
						for id := range a.devices {
							cur.devices[id] = struct{}{}
						}
					}
					mu.Unlock()
				}
			}
		}
	}()

	return d.consume(ctx, "detector", []string{kafkax.TopicTelemetryRaw}, kafka.LastOffset, kafkax.TopicTelemetryDLQ,
		func(ctx context.Context, msgs []kafkax.Message) error {
			var out []Event
			mu.Lock()
			for _, m := range msgs {
				ev, err := decodeTelemetry(m)
				if err != nil {
					continue
				}
				dev, ok := d.Reg.Lookup(ctx, ev.GetDeviceId())
				if !ok {
					continue
				}
				recorded, ingested := telemetryTimes(ev)
				p := ev.GetPoint()
				out = append(out, det.Observe(Observation{
					Dev: dev, Fences: d.Reg.GeofencesFor(dev),
					RecordedAt: recorded, IngestedAt: ingested,
					Lat: p.GetPosition().GetLatitude(), Lon: p.GetPosition().GetLongitude(),
					SpeedKph: p.GetSpeedKph(), Ignition: p.GetIgnitionOn(),
				})...)

				a := usage[dev.TenantID]
				if a == nil {
					a = &usageAcc{devices: map[string]struct{}{}}
					usage[dev.TenantID] = a
				}
				a.points++
				a.devices[dev.DeviceID] = struct{}{}
			}
			mu.Unlock()
			return publish(ctx, out)
		})
}

func publishUsage(ctx context.Context, d Deps, snap map[string]*usageAcc, instance string, seq uint64) error {
	now := time.Now().UTC()
	msgs := make([]kafka.Message, 0, len(snap))
	for tenant, a := range snap {
		b, err := proto.Marshal(&tracklinev1.UsageRecorded{
			Meta:            events.Meta(tenant, detectorSource, "usage:"+tenant+":"+instance+":"+itoa(seq), now),
			UsageDate:       now.Format("2006-01-02"),
			TelemetryPoints: a.points,
			ActiveDevices:   uint32(len(a.devices)),
		})
		if err != nil {
			return err
		}
		msgs = append(msgs, kafka.Message{Topic: kafkax.TopicUsageRecorded, Key: []byte(tenant), Value: b})
	}
	return d.Prod.PublishAll(ctx, msgs)
}

func encodeEvent(e Event) (kafka.Message, error) {
	meta := events.Meta(e.Dev.TenantID, detectorSource, e.Key, e.At)
	pos := &tracklinev1.GeoPoint{Latitude: e.Lat, Longitude: e.Lon}
	key := []byte(e.Dev.VehicleID)

	switch e.Kind {
	case KindGeofence:
		tr := map[string]tracklinev1.GeofenceEvent_Transition{
			"enter": tracklinev1.GeofenceEvent_TRANSITION_ENTER,
			"exit":  tracklinev1.GeofenceEvent_TRANSITION_EXIT,
			"dwell": tracklinev1.GeofenceEvent_TRANSITION_DWELL,
		}[e.Transition]
		b, err := proto.Marshal(&tracklinev1.GeofenceEvent{Meta: meta, VehicleId: e.Dev.VehicleID, GeofenceId: e.Geofence.ID,
			Transition: tr, Position: pos, DwellSeconds: e.DwellSecs})
		return kafka.Message{Topic: kafkax.TopicEventsGeofence, Key: key, Value: b}, err
	case KindSpeeding:
		b, err := proto.Marshal(&tracklinev1.SpeedingEvent{Meta: meta, VehicleId: e.Dev.VehicleID, SpeedKph: e.SpeedKph,
			LimitKph: e.LimitKph, DurationSeconds: e.DurationSecs, Position: pos})
		return kafka.Message{Topic: kafkax.TopicEventsViolation, Key: key, Value: b,
			Headers: []kafka.Header{{Key: kafkax.HeaderEventType, Value: []byte("SpeedingEvent")}}}, err
	case KindIdle:
		b, err := proto.Marshal(&tracklinev1.IdleEvent{Meta: meta, VehicleId: e.Dev.VehicleID, IdleSeconds: e.IdleSecs, Position: pos})
		return kafka.Message{Topic: kafkax.TopicEventsViolation, Key: key, Value: b,
			Headers: []kafka.Header{{Key: kafkax.HeaderEventType, Value: []byte("IdleEvent")}}}, err
	default: // KindOffline
		b, err := proto.Marshal(&tracklinev1.VehicleOfflineEvent{Meta: meta, VehicleId: e.Dev.VehicleID, DeviceId: e.Dev.DeviceID,
			LastSeenAt: e.LastSeen.UnixMilli(), SilentSeconds: e.SilentSecs})
		return kafka.Message{Topic: kafkax.TopicEventsVehicle, Key: key, Value: b}, err
	}
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
