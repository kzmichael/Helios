package io.trackline.streamproc;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import trackline.v1.Events;

class DetectorLogicTest {
    static final String SQUARE = "{\"type\":\"Polygon\",\"coordinates\":[[[72.86,19.06],[72.89,19.06],[72.89,19.09],[72.86,19.09],[72.86,19.06]]]}";
    static final long T0 = 1_800_000_000_000L;

    static Registry.Geofence fence(int dwellSecs) throws Exception {
        return new Registry.Geofence("g1", "t1", "", "Hub", "inclusion", dwellSecs, Polygon.fromGeoJson(SQUARE));
    }

    static Observation obs(long atMs, double lat, double lon, double speed, boolean ignition) {
        Observation o = new Observation();
        o.vehicleId = "v1";
        o.deviceId = "d1";
        o.tenantId = "t1";
        o.fleetId = "f1";
        o.speedLimitKph = 60;
        o.recordedAtMs = atMs;
        o.ingestedAtMs = atMs;
        o.lat = lat;
        o.lon = lon;
        o.speedKph = speed;
        o.ignitionOn = ignition;
        return o;
    }

    @Test
    void polygonContainsAndExcludes() throws Exception {
        Polygon p = Polygon.fromGeoJson(SQUARE);
        assertTrue(p.contains(19.075, 72.875));
        assertFalse(p.contains(19.2, 72.875));
        assertFalse(p.contains(19.075, 72.5));
    }

    @Test
    void firstPointInsideIsABaselineNotAnEntry() throws Exception {
        VehicleState st = new VehicleState();
        List<DetectorLogic.Event> ev = DetectorLogic.observe(st, obs(T0, 19.075, 72.875, 30, true), List.of(fence(0)),
                T0);
        assertTrue(ev.isEmpty());
        assertTrue(st.inside.containsKey("g1"));
    }

    @Test
    void enterThenExitCarryDwellTime() throws Exception {
        VehicleState st = new VehicleState();
        List<Registry.Geofence> fences = List.of(fence(0));
        DetectorLogic.observe(st, obs(T0, 19.2, 72.875, 30, true), fences, T0); // outside: baseline

        List<DetectorLogic.Event> enter = DetectorLogic.observe(st, obs(T0 + 1000, 19.075, 72.875, 30, true), fences,
                T0 + 1000);
        assertEquals(1, enter.size());
        assertEquals("enter", enter.get(0).transition);
        assertEquals("geofence:v1:g1:enter:" + (T0 + 1000), enter.get(0).key);

        List<DetectorLogic.Event> exit = DetectorLogic.observe(st, obs(T0 + 31_000, 19.2, 72.875, 30, true), fences,
                T0 + 31_000);
        assertEquals(1, exit.size());
        assertEquals("exit", exit.get(0).transition);
        assertEquals(30, exit.get(0).dwellSecs);
    }

    @Test
    void dwellIsEmittedOnceAfterTheThreshold() throws Exception {
        VehicleState st = new VehicleState();
        List<Registry.Geofence> fences = List.of(fence(30));
        DetectorLogic.observe(st, obs(T0, 19.2, 72.875, 30, true), fences, T0);
        DetectorLogic.observe(st, obs(T0 + 1000, 19.075, 72.875, 5, true), fences, T0 + 1000);
        assertTrue(DetectorLogic.observe(st, obs(T0 + 20_000, 19.075, 72.875, 5, true), fences, T0 + 20_000).isEmpty());

        List<DetectorLogic.Event> dwell = DetectorLogic.observe(st, obs(T0 + 32_000, 19.075, 72.875, 5, true), fences,
                T0 + 32_000);
        assertEquals(1, dwell.size());
        assertEquals("dwell", dwell.get(0).transition);
        assertTrue(DetectorLogic.observe(st, obs(T0 + 40_000, 19.075, 72.875, 5, true), fences, T0 + 40_000).isEmpty());
    }

    @Test
    void speedingNeedsTenSecondsOverTheLimitAndThenRepeatsEveryTen() {
        VehicleState st = new VehicleState();
        assertTrue(DetectorLogic.observe(st, obs(T0, 19, 72, 80, true), List.of(), T0).isEmpty());
        assertTrue(DetectorLogic.observe(st, obs(T0 + 5000, 19, 72, 80, true), List.of(), T0).isEmpty());

        List<DetectorLogic.Event> first = DetectorLogic.observe(st, obs(T0 + 10_000, 19, 72, 80, true), List.of(), T0);
        assertEquals(1, first.size());
        assertEquals(DetectorLogic.Kind.SPEEDING, first.get(0).kind);
        assertEquals(10, first.get(0).durationSecs);
        assertEquals(60, first.get(0).limitKph);

        assertTrue(DetectorLogic.observe(st, obs(T0 + 15_000, 19, 72, 80, true), List.of(), T0).isEmpty());
        assertEquals(1, DetectorLogic.observe(st, obs(T0 + 20_000, 19, 72, 80, true), List.of(), T0).size());
    }

    @Test
    void droppingBelowTheLimitResetsTheEpisode() {
        VehicleState st = new VehicleState();
        DetectorLogic.observe(st, obs(T0, 19, 72, 80, true), List.of(), T0);
        DetectorLogic.observe(st, obs(T0 + 8000, 19, 72, 40, true), List.of(), T0);
        assertTrue(DetectorLogic.observe(st, obs(T0 + 12_000, 19, 72, 80, true), List.of(), T0).isEmpty());
    }

    @Test
    void idleNeedsAMinuteWithTheIgnitionOn() {
        VehicleState st = new VehicleState();
        DetectorLogic.observe(st, obs(T0, 19, 72, 0, true), List.of(), T0);
        assertTrue(DetectorLogic.observe(st, obs(T0 + 59_000, 19, 72, 0, true), List.of(), T0).isEmpty());
        List<DetectorLogic.Event> idle = DetectorLogic.observe(st, obs(T0 + 60_000, 19, 72, 0, true), List.of(), T0);
        assertEquals(1, idle.size());
        assertEquals(60, idle.get(0).idleSecs);

        VehicleState parked = new VehicleState();
        DetectorLogic.observe(parked, obs(T0, 19, 72, 0, false), List.of(), T0);
        assertTrue(DetectorLogic.observe(parked, obs(T0 + 120_000, 19, 72, 0, false), List.of(), T0).isEmpty());
    }

    @Test
    void redeliveredAndOutOfOrderPointsAreIgnored() {
        VehicleState st = new VehicleState();
        DetectorLogic.observe(st, obs(T0 + 10_000, 19, 72, 80, true), List.of(), T0);
        assertTrue(DetectorLogic.observe(st, obs(T0 + 10_000, 19, 72, 80, true), List.of(), T0).isEmpty());
        assertTrue(DetectorLogic.observe(st, obs(T0, 19, 72, 80, true), List.of(), T0).isEmpty());
        assertEquals(T0 + 10_000, st.lastRecordedMs);
    }

    @Test
    void aClockRunningAheadIsClampedToIngestTime() {
        assertEquals(T0, DetectorLogic.effectiveTime(T0 + 3_600_000, T0));
        assertEquals(T0 + 3000, DetectorLogic.effectiveTime(T0 + 3000, T0));
    }

    @Test
    void offlineAfterAMinuteOfSilenceThenOncePerMinute() {
        VehicleState st = new VehicleState();
        DetectorLogic.observe(st, obs(T0, 19, 72, 30, true), List.of(), T0);

        assertNull(DetectorLogic.offline("v1", st, T0 + 30_000));
        DetectorLogic.Event off = DetectorLogic.offline("v1", st, T0 + 61_000);
        assertNotNull(off);
        assertEquals(61, off.silentSecs);
        assertEquals("d1", off.deviceId);
        assertNull(DetectorLogic.offline("v1", st, T0 + 90_000));
        assertNotNull(DetectorLogic.offline("v1", st, T0 + 122_000));

        // Hearing from the vehicle again clears the offline marker.
        DetectorLogic.observe(st, obs(T0 + 130_000, 19, 72, 30, true), List.of(), T0 + 130_000);
        assertEquals(0, st.lastOfflineEmitMs);
    }

    @Test
    void encodedEventsMatchThePipelineContract() throws Exception {
        VehicleState st = new VehicleState();
        DetectorLogic.observe(st, obs(T0, 19, 72, 80, true), List.of(), T0);
        DetectorLogic.Event speeding = DetectorLogic.observe(st, obs(T0 + 10_000, 19, 72, 80, true), List.of(), T0)
                .get(0);

        OutRecord r = OutRecord.encode(speeding);
        assertEquals("trackline.events.violation", r.topic);
        assertEquals("v1", r.key);
        assertEquals("SpeedingEvent", r.eventType);
        Events.SpeedingEvent decoded = Events.SpeedingEvent.parseFrom(r.value);
        assertEquals("t1", decoded.getMeta().getTenantId());
        assertEquals("stream-proc", decoded.getMeta().getSource());
        assertEquals(speeding.key, decoded.getMeta().getIdempotencyKey());
        assertEquals(80, decoded.getSpeedKph());
    }

    @Test
    void usageKeyIsDerivedFromTheWindowSoReplaysDeduplicate() throws Exception {
        OutRecord a = OutRecord.usage("t1", T0, T0 + 30_000, 120, 4);
        OutRecord b = OutRecord.usage("t1", T0, T0 + 30_000, 120, 4);
        Events.UsageRecorded ua = Events.UsageRecorded.parseFrom(a.value);
        Events.UsageRecorded ub = Events.UsageRecorded.parseFrom(b.value);
        assertEquals(ua.getMeta().getIdempotencyKey(), ub.getMeta().getIdempotencyKey());
        assertEquals("usage:t1:" + T0, ua.getMeta().getIdempotencyKey());
        assertEquals(120, ua.getTelemetryPoints());
        assertEquals(4, ua.getActiveDevices());
        assertEquals("trackline.usage.recorded", a.topic);
        assertEquals("t1", a.key);
    }

    @Test
    void databaseUrlBecomesJdbc() {
        Config c = Config
                .of(Map.of("DATABASE_URL", "postgres://myuser:p%40ss@db.internal:6543/trackline?sslmode=disable"));
        assertEquals("jdbc:postgresql://db.internal:6543/trackline?sslmode=disable", c.jdbcUrl);
        assertEquals("myuser", c.dbUser);
        assertEquals("p@ss", c.dbPassword);
        assertFalse(c.toString().contains("p@ss"));
        assertEquals("stream-processor", c.groupId);
    }
}
