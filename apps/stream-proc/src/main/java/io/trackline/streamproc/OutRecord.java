package io.trackline.streamproc;

import java.util.UUID;
import trackline.v1.Common;
import trackline.v1.Events;

/**
 * A message ready for Kafka: which topic, which key, which bytes, and the
 * event-type header.
 */
public class OutRecord {
    public static final String TOPIC_GEOFENCE = "trackline.events.geofence";
    public static final String TOPIC_VIOLATION = "trackline.events.violation";
    public static final String TOPIC_VEHICLE = "trackline.events.vehicle";
    public static final String TOPIC_USAGE = "trackline.usage.recorded";
    /**
     * Header that tells a consumer of events.violation which message type it holds.
     */
    public static final String HEADER_EVENT_TYPE = "x-event-type";
    static final String SOURCE = "stream-proc";

    public String topic;
    public String key;
    public byte[] value;
    public String eventType; // may be null

    public OutRecord() {
    }

    OutRecord(String topic, String key, byte[] value, String eventType) {
        this.topic = topic;
        this.key = key;
        this.value = value;
        this.eventType = eventType;
    }

    static Common.EventMeta meta(String tenantId, String idempotencyKey, long occurredAtMs) {
        return Common.EventMeta.newBuilder()
                .setEventId(UUID.randomUUID().toString())
                .setTenantId(tenantId == null ? "" : tenantId)
                .setSource(SOURCE)
                .setOccurredAt(occurredAtMs)
                .setIdempotencyKey(idempotencyKey)
                .build();
    }

    /**
     * Encodes a detector event with the same schema, topic, key and headers as
     * apps/pipeline.
     */
    public static OutRecord encode(DetectorLogic.Event e) {
        Common.EventMeta meta = meta(e.tenantId, e.key, e.atMs);
        Common.GeoPoint pos = Common.GeoPoint.newBuilder().setLatitude(e.lat).setLongitude(e.lon).build();

        switch (e.kind) {
            case GEOFENCE: {
                Events.GeofenceEvent.Transition tr = switch (e.transition) {
                    case "enter" -> Events.GeofenceEvent.Transition.TRANSITION_ENTER;
                    case "exit" -> Events.GeofenceEvent.Transition.TRANSITION_EXIT;
                    default -> Events.GeofenceEvent.Transition.TRANSITION_DWELL;
                };
                byte[] b = Events.GeofenceEvent.newBuilder().setMeta(meta).setVehicleId(e.vehicleId)
                        .setGeofenceId(e.geofenceId)
                        .setTransition(tr).setPosition(pos).setDwellSeconds(e.dwellSecs).build().toByteArray();
                return new OutRecord(TOPIC_GEOFENCE, e.vehicleId, b, null);
            }
            case SPEEDING: {
                byte[] b = Events.SpeedingEvent.newBuilder().setMeta(meta).setVehicleId(e.vehicleId)
                        .setSpeedKph(e.speedKph)
                        .setLimitKph(e.limitKph).setDurationSeconds(e.durationSecs).setPosition(pos).build()
                        .toByteArray();
                return new OutRecord(TOPIC_VIOLATION, e.vehicleId, b, "SpeedingEvent");
            }
            case IDLE: {
                byte[] b = Events.IdleEvent.newBuilder().setMeta(meta).setVehicleId(e.vehicleId)
                        .setIdleSeconds(e.idleSecs)
                        .setPosition(pos).build().toByteArray();
                return new OutRecord(TOPIC_VIOLATION, e.vehicleId, b, "IdleEvent");
            }
            default: {
                byte[] b = Events.VehicleOfflineEvent.newBuilder().setMeta(meta).setVehicleId(e.vehicleId)
                        .setDeviceId(e.deviceId == null ? "" : e.deviceId).setLastSeenAt(e.lastSeenMs)
                        .setSilentSeconds(e.silentSecs)
                        .build().toByteArray();
                return new OutRecord(TOPIC_VEHICLE, e.vehicleId, b, null);
            }
        }
    }

    /**
     * One window of one tenant's usage. The idempotency key is derived from the
     * window,
     * not from a counter, so reprocessing the same window after a restart or a
     * replay
     * produces the same key and the usage collector drops the duplicate.
     */
    public static OutRecord usage(String tenantId, long windowStartMs, long windowEndMs, long points,
            int activeDevices) {
        String date = java.time.Instant.ofEpochMilli(windowStartMs).atZone(java.time.ZoneOffset.UTC).toLocalDate()
                .toString();
        Common.EventMeta meta = meta(tenantId, "usage:" + tenantId + ":" + windowStartMs, windowEndMs);
        byte[] b = Events.UsageRecorded.newBuilder().setMeta(meta).setUsageDate(date).setTelemetryPoints(points)
                .setActiveDevices(activeDevices).build().toByteArray();
        return new OutRecord(TOPIC_USAGE, tenantId, b, null);
    }
}
