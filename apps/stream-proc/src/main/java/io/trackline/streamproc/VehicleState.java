package io.trackline.streamproc;

import java.util.HashMap;
import java.util.Map;

/** Per-vehicle detector state, kept in Flink keyed state. Zero means "not set". */
public class VehicleState {
    public String deviceId;
    public String tenantId;

    public long lastRecordedMs;
    public long lastSeenMs;

    public boolean baselined;
    public Map<String, FenceState> inside = new HashMap<>();

    public long overSinceMs;
    public long lastSpeedingEmitMs;

    public long idleSinceMs;
    public long lastIdleEmitMs;

    public long lastOfflineEmitMs;

    /** The processing-time timer currently registered for this key, if any. */
    public long nextTimerMs;

    public VehicleState() {}
}
