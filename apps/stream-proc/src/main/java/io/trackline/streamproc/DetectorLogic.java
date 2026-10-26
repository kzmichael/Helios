package io.trackline.streamproc;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * The per-vehicle state machine behind geofence, speeding, idle and offline
 * events.
 * It is a direct port of the Phase 0 detector in apps/pipeline so the two emit
 * the
 * same events with the same idempotency keys; only one of them should run.
 *
 * Pure: no clock, no Flink, no I/O. Time comes in as arguments.
 */
public final class DetectorLogic {
    // Base-event thresholds. Deliberately lower than any tenant's rule: the alert
    // engine applies rule parameters (tolerance, minimum duration, idle and offline
    // thresholds), not this job.
    static final long SPEEDING_MIN_DURATION_MS = 10_000;
    static final long SPEEDING_REPEAT_MS = 10_000;
    static final double IDLE_SPEED_KPH = 2.0;
    static final long IDLE_MIN_DURATION_MS = 60_000;
    static final long IDLE_REPEAT_MS = 60_000;
    static final long OFFLINE_AFTER_MS = 60_000;
    static final long OFFLINE_REPEAT_MS = 60_000;
    /** A device clock further ahead than this is clamped to ingest time. */
    static final long MAX_FUTURE_SKEW_MS = 5_000;

    public enum Kind {
        GEOFENCE, SPEEDING, IDLE, OFFLINE
    }

    /** A detector output, before it is encoded for Kafka. */
    public static final class Event {
        public Kind kind;
        public String vehicleId;
        public String deviceId;
        public String tenantId;
        public long atMs;
        public double lat;
        public double lon;
        public String key;

        public String geofenceId;
        public String transition; // enter | exit | dwell
        public long dwellSecs;

        public double speedKph;
        public double limitKph;
        public long durationSecs;

        public long idleSecs;

        public long silentSecs;
        public long lastSeenMs;
    }

    private DetectorLogic() {
    }

    public static long effectiveTime(long recordedMs, long ingestedMs) {
        if (ingestedMs > 0 && recordedMs > ingestedMs + MAX_FUTURE_SKEW_MS) {
            return ingestedMs;
        }
        return recordedMs;
    }

    /**
     * Folds one telemetry point into the vehicle's state and returns any events it
     * triggers. Points not newer than the last one are ignored, which also makes
     * redelivered and duplicated points harmless.
     */
    public static List<Event> observe(VehicleState st, Observation o, List<Registry.Geofence> fences, long nowMs) {
        long at = effectiveTime(o.recordedAtMs, o.ingestedAtMs);
        if (st.lastRecordedMs != 0 && at <= st.lastRecordedMs) {
            return List.of();
        }
        st.deviceId = o.deviceId;
        st.tenantId = o.tenantId;
        st.lastRecordedMs = at;
        st.lastSeenMs = nowMs;
        st.lastOfflineEmitMs = 0;

        List<Event> out = new ArrayList<>();
        geofences(st, o, fences, at, out);
        speeding(st, o, at, out);
        idle(st, o, at, out);
        st.baselined = true;
        return out;
    }

    private static Event base(Kind kind, Observation o, long at, String key) {
        Event e = new Event();
        e.kind = kind;
        e.vehicleId = o.vehicleId;
        e.deviceId = o.deviceId;
        e.tenantId = o.tenantId;
        e.atMs = at;
        e.lat = o.lat;
        e.lon = o.lon;
        e.key = key;
        return e;
    }

    private static void geofences(VehicleState st, Observation o, List<Registry.Geofence> fences, long at,
            List<Event> out) {
        Set<String> current = new HashSet<>();
        for (Registry.Geofence g : fences) {
            current.add(g.id());
            boolean in = g.boundary().contains(o.lat, o.lon);
            FenceState fs = st.inside.get(g.id());
            boolean was = fs != null;

            if (in && !was) {
                st.inside.put(g.id(), new FenceState(at));
                // The first point ever seen for a vehicle is a baseline, not a transition:
                // we do not know where it was before.
                if (st.baselined) {
                    out.add(fenceEvent(o, g, "enter", at, 0));
                }
            } else if (!in && was) {
                st.inside.remove(g.id());
                out.add(fenceEvent(o, g, "exit", at, (at - fs.sinceMs) / 1000));
            } else if (in && was && !fs.dwellEmitted && g.dwellSecs() > 0 && at - fs.sinceMs >= g.dwellSecs() * 1000L) {
                fs.dwellEmitted = true;
                out.add(fenceEvent(o, g, "dwell", at, (at - fs.sinceMs) / 1000));
            }
        }
        // A geofence removed or disabled while the vehicle was inside it.
        st.inside.keySet().removeIf(id -> !current.contains(id));
    }

    private static Event fenceEvent(Observation o, Registry.Geofence g, String transition, long at, long dwellSecs) {
        Event e = base(Kind.GEOFENCE, o, at, "geofence:" + o.vehicleId + ":" + g.id() + ":" + transition + ":" + at);
        e.geofenceId = g.id();
        e.transition = transition;
        e.dwellSecs = dwellSecs;
        return e;
    }

    private static void speeding(VehicleState st, Observation o, long at, List<Event> out) {
        if (o.speedKph <= o.speedLimitKph) {
            st.overSinceMs = 0;
            st.lastSpeedingEmitMs = 0;
            return;
        }
        if (st.overSinceMs == 0) {
            st.overSinceMs = at;
        }
        long over = at - st.overSinceMs;
        if (over < SPEEDING_MIN_DURATION_MS) {
            return;
        }
        if (st.lastSpeedingEmitMs != 0 && at - st.lastSpeedingEmitMs < SPEEDING_REPEAT_MS) {
            return;
        }
        st.lastSpeedingEmitMs = at;
        Event e = base(Kind.SPEEDING, o, at,
                "speeding:" + o.vehicleId + ":" + (at / 1000) / (SPEEDING_REPEAT_MS / 1000));
        e.speedKph = o.speedKph;
        e.limitKph = o.speedLimitKph;
        e.durationSecs = over / 1000;
        out.add(e);
    }

    private static void idle(VehicleState st, Observation o, long at, List<Event> out) {
        if (!o.ignitionOn || o.speedKph >= IDLE_SPEED_KPH) {
            st.idleSinceMs = 0;
            st.lastIdleEmitMs = 0;
            return;
        }
        if (st.idleSinceMs == 0) {
            st.idleSinceMs = at;
        }
        long idle = at - st.idleSinceMs;
        if (idle < IDLE_MIN_DURATION_MS) {
            return;
        }
        if (st.lastIdleEmitMs != 0 && at - st.lastIdleEmitMs < IDLE_REPEAT_MS) {
            return;
        }
        st.lastIdleEmitMs = at;
        Event e = base(Kind.IDLE, o, at, "idle:" + o.vehicleId + ":" + (at / 1000) / (IDLE_REPEAT_MS / 1000));
        e.idleSecs = idle / 1000;
        out.add(e);
    }

    /**
     * Emits an offline event if the vehicle has gone quiet, at most once per
     * OFFLINE_REPEAT_MS. Returns null when it has not.
     */
    public static Event offline(String vehicleId, VehicleState st, long nowMs) {
        if (st.lastSeenMs == 0) {
            return null;
        }
        long silent = nowMs - st.lastSeenMs;
        if (silent < OFFLINE_AFTER_MS) {
            return null;
        }
        if (st.lastOfflineEmitMs != 0 && nowMs - st.lastOfflineEmitMs < OFFLINE_REPEAT_MS) {
            return null;
        }
        st.lastOfflineEmitMs = nowMs;

        Event e = new Event();
        e.kind = Kind.OFFLINE;
        e.vehicleId = vehicleId;
        e.deviceId = st.deviceId;
        e.tenantId = st.tenantId;
        e.atMs = nowMs;
        e.silentSecs = silent / 1000;
        e.lastSeenMs = st.lastSeenMs;
        e.key = "offline:" + vehicleId + ":" + (nowMs / 1000) / (OFFLINE_REPEAT_MS / 1000);
        return e;
    }
}
