package roles

import (
	"strings"
	"testing"
	"time"

	"github.com/helios/trackline/apps/pipeline/internal/events"
)

// Rules below mirror apps/_db/seed/V2__seed_tenants.sql.
var (
	atlasSpeeding = Rule{ID: "r1", Type: "speeding", Severity: "warning", CooldownS: 300,
		Params: map[string]any{"tolerance_kph": 5.0, "min_duration_s": 15.0}}
	meridianPort = Rule{ID: "r2", Type: "geofence_enter", Severity: "critical", CooldownS: 60,
		Params: map[string]any{"geofence_id": "9f000000-0000-4000-8000-000000000003"}}
	atlasOffline = Rule{ID: "r3", Type: "offline", Severity: "critical", CooldownS: 900,
		Params: map[string]any{"silent_seconds": 300.0}}
	atlasIdle = Rule{ID: "r4", Type: "idle", Severity: "warning", CooldownS: 3600,
		Params: map[string]any{"idle_seconds": 1200.0}}
)

func TestSpeedingRuleAppliesToleranceAndDuration(t *testing.T) {
	cases := []struct {
		name string
		c    Candidate
		want bool
	}{
		{"over tolerance and long enough", Candidate{Type: "speeding", SpeedKph: 90, LimitKph: 70, DurationSecs: 20}, true},
		{"within tolerance", Candidate{Type: "speeding", SpeedKph: 74, LimitKph: 70, DurationSecs: 20}, false},
		{"not long enough", Candidate{Type: "speeding", SpeedKph: 90, LimitKph: 70, DurationSecs: 10}, false},
		{"wrong event type", Candidate{Type: "idle", IdleSecs: 9999}, false},
	}
	for _, tc := range cases {
		if got := atlasSpeeding.Matches(tc.c); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestGeofenceRuleIsScopedToItsGeofence(t *testing.T) {
	port := Candidate{Type: "geofence_enter", GeofenceID: "9f000000-0000-4000-8000-000000000003"}
	hub := Candidate{Type: "geofence_enter", GeofenceID: "9f000000-0000-4000-8000-000000000002"}
	if !meridianPort.Matches(port) {
		t.Fatal("rule should match its own geofence")
	}
	if meridianPort.Matches(hub) {
		t.Fatal("rule must not match a different geofence")
	}
	anyFence := Rule{Type: "geofence_exit", Params: map[string]any{}}
	if !anyFence.Matches(Candidate{Type: "geofence_exit", GeofenceID: "whatever"}) {
		t.Fatal("a rule without geofence_id matches every geofence")
	}
}

func TestFleetScopedRuleOnlyMatchesThatFleet(t *testing.T) {
	r := Rule{Type: "geofence_dwell", FleetID: "f4", Params: map[string]any{}}
	if !r.Matches(Candidate{Type: "geofence_dwell", FleetID: "f4"}) || r.Matches(Candidate{Type: "geofence_dwell", FleetID: "f1"}) {
		t.Fatal("fleet scoping not honoured")
	}
}

func TestOfflineAndIdleThresholds(t *testing.T) {
	if atlasOffline.Matches(Candidate{Type: "offline", SilentSecs: 120}) {
		t.Fatal("120s of silence is under the 300s rule")
	}
	if !atlasOffline.Matches(Candidate{Type: "offline", SilentSecs: 300}) {
		t.Fatal("300s of silence meets the rule")
	}
	if atlasIdle.Matches(Candidate{Type: "idle", IdleSecs: 600}) || !atlasIdle.Matches(Candidate{Type: "idle", IdleSecs: 1200}) {
		t.Fatal("idle threshold not applied")
	}
}

func TestChannelSeveritySubscription(t *testing.T) {
	warn := Channel{MinSeverity: "warning"}
	if warn.WantsSeverity("info") || !warn.WantsSeverity("warning") || !warn.WantsSeverity("critical") {
		t.Fatal("a warning channel wants warning and above")
	}
	sms := Channel{MinSeverity: "critical"}
	if sms.WantsSeverity("warning") || !sms.WantsSeverity("critical") {
		t.Fatal("a critical channel wants critical only")
	}
}

func TestRecipientPerChannelKind(t *testing.T) {
	if (Channel{Kind: "webhook", Config: map[string]any{"url": "http://sink/x"}}).Recipient() != "http://sink/x" {
		t.Fatal("webhook recipient is the url")
	}
	if (Channel{Kind: "email", Config: map[string]any{"to": "ops@x.test"}}).Recipient() != "ops@x.test" {
		t.Fatal("email recipient is the to address")
	}
}

func TestSummaries(t *testing.T) {
	got := Summary(Candidate{Type: "geofence_enter"}, "MH-01", "Restricted Port Area")
	if got != "MH-01 entered Restricted Port Area" {
		t.Fatalf("unexpected summary %q", got)
	}
	if s := Summary(Candidate{Type: "speeding", SpeedKph: 95, LimitKph: 70, DurationSecs: 30}, "DL-02", ""); !strings.Contains(s, "95 km/h") || !strings.Contains(s, "limit 70") {
		t.Fatalf("unexpected speeding summary %q", s)
	}
}

func TestDeliveryStatusMapping(t *testing.T) {
	if DeliveryStatus(true, 1) != "sent" || DeliveryStatus(false, 2) != "failed" || DeliveryStatus(false, 5) != "dead" {
		t.Fatal("unexpected delivery status mapping")
	}
}

func TestWebhookBodyIsJSONAndEmailIsText(t *testing.T) {
	c := Candidate{TenantID: "t", VehicleID: "v", Type: "speeding", At: time.Unix(1700000000, 0)}
	if b := notificationBody("webhook", "a1", "warning", "s", "KA-01", c, `{"k":1}`); !strings.HasPrefix(b, "{") || !strings.Contains(b, `"alertId":"a1"`) {
		t.Fatalf("webhook body should be JSON, got %q", b)
	}
	if b := notificationBody("email", "a1", "warning", "headline", "KA-01", c, `{}`); !strings.HasPrefix(b, "headline") {
		t.Fatalf("email body should lead with the summary, got %q", b)
	}
}

func TestAlertIDsAreStableAndDistinctPerRule(t *testing.T) {
	a, b := events.UUID5("event-1|rule-1"), events.UUID5("event-1|rule-1")
	if a != b {
		t.Fatal("the same event and rule must give the same alert id")
	}
	if a == events.UUID5("event-1|rule-2") {
		t.Fatal("different rules must give different alert ids")
	}
	if len(a) != 36 || a[14] != '5' {
		t.Fatalf("expected a version 5 uuid, got %s", a)
	}
}
