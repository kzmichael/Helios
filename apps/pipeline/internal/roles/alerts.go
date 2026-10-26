package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	"github.com/helios/trackline/apps/pipeline/internal/events"
	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

const alertSource = "pipeline/alert-engine"

type Rule struct {
	ID, TenantID, FleetID, Name, Type, Severity string
	Params                                      map[string]any
	CooldownS                                   int
}

type Channel struct {
	ID, TenantID, Name, Kind, MinSeverity string
	Config                                map[string]any
}

// Candidate is a normalised detector event the engine evaluates rules against.
type Candidate struct {
	Type       string // geofence_enter | geofence_exit | geofence_dwell | speeding | idle | offline
	TenantID   string
	VehicleID  string
	FleetID    string
	GeofenceID string
	At         time.Time
	IdemKey    string
	Lat, Lon   float64

	SpeedKph, LimitKph float64
	DurationSecs       int64
	IdleSecs           int64
	SilentSecs         int64
	DwellSecs          int64
}

func (r Rule) param(name string, fallback float64) float64 {
	if v, ok := r.Params[name].(float64); ok {
		return v
	}
	return fallback
}

// Matches applies the rule's own parameters. The detector emits base events at low
// thresholds; whether one is alert-worthy is the tenant's rule, decided here.
func (r Rule) Matches(c Candidate) bool {
	if r.Type != c.Type || (r.FleetID != "" && r.FleetID != c.FleetID) {
		return false
	}
	switch c.Type {
	case "geofence_enter", "geofence_exit", "geofence_dwell":
		gid, _ := r.Params["geofence_id"].(string)
		return gid == "" || gid == c.GeofenceID
	case "speeding":
		return c.SpeedKph > c.LimitKph+r.param("tolerance_kph", 0) && float64(c.DurationSecs) >= r.param("min_duration_s", 0)
	case "idle":
		return float64(c.IdleSecs) >= r.param("idle_seconds", 0)
	case "offline":
		return float64(c.SilentSecs) >= r.param("silent_seconds", 0)
	}
	return false
}

var severityRank = map[string]int{"info": 1, "warning": 2, "critical": 3}

// WantsSeverity reports whether the channel is subscribed to alerts of this severity.
func (ch Channel) WantsSeverity(sev string) bool {
	return severityRank[sev] >= severityRank[ch.MinSeverity]
}

func (ch Channel) configString(key string) string {
	s, _ := ch.Config[key].(string)
	return s
}

// Recipient is where the notifier should deliver: an address, a URL, or a number.
func (ch Channel) Recipient() string {
	if ch.Kind == "webhook" {
		return ch.configString("url")
	}
	return ch.configString("to")
}

// Summary is the one-line, human-readable description of an alert.
func Summary(c Candidate, registration, geofenceName string) string {
	switch c.Type {
	case "geofence_enter":
		return fmt.Sprintf("%s entered %s", registration, geofenceName)
	case "geofence_exit":
		return fmt.Sprintf("%s left %s", registration, geofenceName)
	case "geofence_dwell":
		return fmt.Sprintf("%s has been in %s for %d min", registration, geofenceName, c.DwellSecs/60)
	case "speeding":
		return fmt.Sprintf("%s travelling at %.0f km/h (limit %.0f) for %ds", registration, c.SpeedKph, c.LimitKph, c.DurationSecs)
	case "idle":
		return fmt.Sprintf("%s has been idling for %d min", registration, c.IdleSecs/60)
	default:
		return fmt.Sprintf("%s has not reported for %d min", registration, c.SilentSecs/60)
	}
}

type ruleCache struct {
	mu       sync.Mutex
	loaded   time.Time
	rules    map[string][]Rule
	channels map[string][]Channel
	byChan   map[string]Channel
}

func (d Deps) loadRules(ctx context.Context, c *ruleCache) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.loaded) < 30*time.Second {
		return nil
	}

	rules := map[string][]Rule{}
	rows, err := d.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, coalesce(fleet_id::text, ''), name, alert_type, severity, params::text, cooldown_s
		FROM alert_rule WHERE enabled`)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}
	for rows.Next() {
		var r Rule
		var params string
		if err := rows.Scan(&r.ID, &r.TenantID, &r.FleetID, &r.Name, &r.Type, &r.Severity, &params, &r.CooldownS); err != nil {
			rows.Close()
			return err
		}
		_ = json.Unmarshal([]byte(params), &r.Params)
		rules[r.TenantID] = append(rules[r.TenantID], r)
	}
	rows.Close()

	channels, byChan := map[string][]Channel{}, map[string]Channel{}
	crows, err := d.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, name, kind, min_severity, config::text
		FROM notification_channel WHERE enabled`)
	if err != nil {
		return fmt.Errorf("load channels: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var ch Channel
		var cfg string
		if err := crows.Scan(&ch.ID, &ch.TenantID, &ch.Name, &ch.Kind, &ch.MinSeverity, &cfg); err != nil {
			return err
		}
		_ = json.Unmarshal([]byte(cfg), &ch.Config)
		channels[ch.TenantID] = append(channels[ch.TenantID], ch)
		byChan[ch.ID] = ch
	}

	c.rules, c.channels, c.byChan, c.loaded = rules, channels, byChan, time.Now()
	return nil
}

// decodeCandidate normalises an events.* message.
func decodeCandidate(m kafkax.Message) (Candidate, error) {
	header := ""
	for _, h := range m.Headers {
		if h.Key == kafkax.HeaderEventType {
			header = string(h.Value)
		}
	}

	switch m.Topic {
	case kafkax.TopicEventsGeofence:
		var e tracklinev1.GeofenceEvent
		if err := proto.Unmarshal(m.Value, &e); err != nil {
			return Candidate{}, err
		}
		t := map[tracklinev1.GeofenceEvent_Transition]string{
			tracklinev1.GeofenceEvent_TRANSITION_ENTER: "geofence_enter",
			tracklinev1.GeofenceEvent_TRANSITION_EXIT:  "geofence_exit",
			tracklinev1.GeofenceEvent_TRANSITION_DWELL: "geofence_dwell",
		}[e.GetTransition()]
		if t == "" {
			return Candidate{}, fmt.Errorf("unknown geofence transition")
		}
		return Candidate{Type: t, TenantID: e.GetMeta().GetTenantId(), VehicleID: e.GetVehicleId(), GeofenceID: e.GetGeofenceId(),
			At: time.UnixMilli(e.GetMeta().GetOccurredAt()), IdemKey: e.GetMeta().GetIdempotencyKey(),
			Lat: e.GetPosition().GetLatitude(), Lon: e.GetPosition().GetLongitude(), DwellSecs: e.GetDwellSeconds()}, nil

	case kafkax.TopicEventsViolation:
		switch header {
		case "SpeedingEvent":
			var e tracklinev1.SpeedingEvent
			if err := proto.Unmarshal(m.Value, &e); err != nil {
				return Candidate{}, err
			}
			return Candidate{Type: "speeding", TenantID: e.GetMeta().GetTenantId(), VehicleID: e.GetVehicleId(),
				At: time.UnixMilli(e.GetMeta().GetOccurredAt()), IdemKey: e.GetMeta().GetIdempotencyKey(),
				Lat: e.GetPosition().GetLatitude(), Lon: e.GetPosition().GetLongitude(),
				SpeedKph: e.GetSpeedKph(), LimitKph: e.GetLimitKph(), DurationSecs: e.GetDurationSeconds()}, nil
		case "IdleEvent":
			var e tracklinev1.IdleEvent
			if err := proto.Unmarshal(m.Value, &e); err != nil {
				return Candidate{}, err
			}
			return Candidate{Type: "idle", TenantID: e.GetMeta().GetTenantId(), VehicleID: e.GetVehicleId(),
				At: time.UnixMilli(e.GetMeta().GetOccurredAt()), IdemKey: e.GetMeta().GetIdempotencyKey(),
				Lat: e.GetPosition().GetLatitude(), Lon: e.GetPosition().GetLongitude(), IdleSecs: e.GetIdleSeconds()}, nil
		}
		return Candidate{}, fmt.Errorf("violation message has unknown %s %q", kafkax.HeaderEventType, header)

	case kafkax.TopicEventsVehicle:
		var e tracklinev1.VehicleOfflineEvent
		if err := proto.Unmarshal(m.Value, &e); err != nil {
			return Candidate{}, err
		}
		return Candidate{Type: "offline", TenantID: e.GetMeta().GetTenantId(), VehicleID: e.GetVehicleId(),
			At: time.UnixMilli(e.GetMeta().GetOccurredAt()), IdemKey: e.GetMeta().GetIdempotencyKey(), SilentSecs: e.GetSilentSeconds()}, nil
	}
	return Candidate{}, fmt.Errorf("unexpected topic %q", m.Topic)
}

// AlertEngine is the alert-engine consumer group. For each detector event it
// finds the tenant's matching rules, applies the per-rule cooldown, persists the
// alert, and requests a notification on every channel subscribed to its severity.
func AlertEngine(ctx context.Context, d Deps) error {
	cache := &ruleCache{}
	topics := []string{kafkax.TopicEventsGeofence, kafkax.TopicEventsViolation, kafkax.TopicEventsVehicle}

	return d.consume(ctx, "alert-engine", topics, kafka.FirstOffset, "", func(ctx context.Context, msgs []kafkax.Message) error {
		if err := d.loadRules(ctx, cache); err != nil {
			return err
		}
		var out []kafka.Message
		for _, m := range msgs {
			c, err := decodeCandidate(m)
			if err != nil {
				d.Log.Warn("skipping undecodable event", "role", "alert-engine", "topic", m.Topic, "error", err)
				continue
			}
			dev, ok := d.Reg.Vehicle(ctx, c.VehicleID)
			if !ok {
				continue
			}
			c.FleetID = dev.FleetID

			cache.mu.Lock()
			rules, channels := cache.rules[c.TenantID], cache.channels[c.TenantID]
			cache.mu.Unlock()

			for _, r := range rules {
				if !r.Matches(c) {
					continue
				}
				msgsOut, err := d.raise(ctx, r, channels, c, dev.Registration)
				if err != nil {
					return err
				}
				out = append(out, msgsOut...)
			}
		}
		return d.Prod.PublishAll(ctx, out)
	})
}

// raise records one alert and returns the Kafka messages to publish for it. It is
// safe to call again for the same event: the alert id is derived from the event's
// idempotency key and each delivery is unique per (alert, channel).
func (d Deps) raise(ctx context.Context, r Rule, channels []Channel, c Candidate, registration string) ([]kafka.Message, error) {
	alertID := events.UUID5(c.IdemKey + "|" + r.ID)

	var onCooldown bool
	if err := d.Pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM alert
		               WHERE tenant_id = $1::uuid AND rule_id = $2::uuid AND vehicle_id = $3::uuid
		                 AND id <> $4::uuid AND triggered_at > $5)`,
		c.TenantID, r.ID, c.VehicleID, alertID, c.At.Add(-time.Duration(r.CooldownS)*time.Second)).Scan(&onCooldown); err != nil {
		return nil, fmt.Errorf("cooldown check: %w", err)
	}
	if onCooldown {
		return nil, nil
	}

	geofenceName := ""
	if g, ok := d.Reg.Geofence(c.GeofenceID); ok {
		geofenceName = g.Name
	}
	summary := Summary(c, registration, geofenceName)
	payload, _ := json.Marshal(map[string]any{
		"registration": registration, "geofenceName": geofenceName, "speedKph": c.SpeedKph, "limitKph": c.LimitKph,
		"durationSeconds": c.DurationSecs, "idleSeconds": c.IdleSecs, "silentSeconds": c.SilentSecs, "dwellSeconds": c.DwellSecs,
	})

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `
		INSERT INTO alert (id, tenant_id, rule_id, vehicle_id, geofence_id, alert_type, severity, summary, payload,
		                   position_lat, position_lon, triggered_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, NULLIF($5, '')::uuid, $6, $7, $8, $9::jsonb, $10, $11, $12)
		ON CONFLICT (id) DO NOTHING`,
		alertID, c.TenantID, r.ID, c.VehicleID, c.GeofenceID, c.Type, r.Severity, summary, string(payload), c.Lat, c.Lon, c.At)
	if err != nil {
		return nil, fmt.Errorf("insert alert: %w", err)
	}
	isNew := tag.RowsAffected() == 1

	for _, ch := range channels {
		if !ch.WantsSeverity(r.Severity) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO notification_delivery (tenant_id, alert_id, channel_id, idempotency_key)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4)
			ON CONFLICT (idempotency_key) DO NOTHING`,
			c.TenantID, alertID, ch.ID, alertID+":"+ch.ID); err != nil {
			return nil, fmt.Errorf("insert delivery: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	var out []kafka.Message
	if isNew {
		b, err := proto.Marshal(&tracklinev1.AlertRaised{
			Meta:    events.Meta(c.TenantID, alertSource, "alert:"+alertID, c.At),
			AlertId: alertID, RuleId: r.ID, VehicleId: c.VehicleID, AlertType: c.Type,
			Severity: severityEnum(r.Severity), Summary: summary, PayloadJson: string(payload),
		})
		if err != nil {
			return nil, err
		}
		out = append(out, kafka.Message{Topic: kafkax.TopicAlertsRaised, Key: []byte(c.TenantID), Value: b})
	}

	// Anything still pending with no attempts is unsent, including a notification
	// whose publish was lost to a crash between commit and publish.
	rows, err := d.Pool.Query(ctx, `
		SELECT channel_id::text, idempotency_key FROM notification_delivery
		WHERE alert_id = $1::uuid AND status = 'pending' AND attempts = 0`, alertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]Channel{}
	for _, ch := range channels {
		byID[ch.ID] = ch
	}
	for rows.Next() {
		var chID, key string
		if err := rows.Scan(&chID, &key); err != nil {
			return nil, err
		}
		ch, ok := byID[chID]
		if !ok {
			continue
		}
		b, err := proto.Marshal(&tracklinev1.NotificationRequested{
			Meta:    events.Meta(c.TenantID, alertSource, key, c.At),
			AlertId: alertID, ChannelId: ch.ID, ChannelKind: ch.Kind, Recipient: ch.Recipient(),
			Subject: fmt.Sprintf("[%s] %s", strings.ToUpper(r.Severity), summary),
			Body:    notificationBody(ch.Kind, alertID, r.Severity, summary, registration, c, string(payload)),
			Attempt: 1,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, kafka.Message{Topic: kafkax.TopicNotificationsReq, Key: []byte(alertID), Value: b})
	}
	return out, rows.Err()
}

// notificationBody is plain text for email and SMS, and a JSON document for webhooks.
func notificationBody(kind, alertID, severity, summary, registration string, c Candidate, payload string) string {
	if kind == "webhook" {
		b, _ := json.Marshal(map[string]any{
			"alertId": alertID, "tenantId": c.TenantID, "vehicleId": c.VehicleID, "registration": registration,
			"type": c.Type, "severity": severity, "summary": summary, "triggeredAt": c.At.UTC().Format(time.RFC3339),
			"position": map[string]float64{"lat": c.Lat, "lon": c.Lon}, "details": json.RawMessage(payload),
		})
		return string(b)
	}
	return fmt.Sprintf("%s\n\nSeverity: %s\nTime: %s\nVehicle: %s\nPosition: %.5f, %.5f\nAlert id: %s\n",
		summary, severity, c.At.UTC().Format(time.RFC1123), registration, c.Lat, c.Lon, alertID)
}

func severityEnum(s string) tracklinev1.Severity {
	switch s {
	case "critical":
		return tracklinev1.Severity_SEVERITY_CRITICAL
	case "warning":
		return tracklinev1.Severity_SEVERITY_WARNING
	default:
		return tracklinev1.Severity_SEVERITY_INFO
	}
}
