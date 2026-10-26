package io.trackline.streamproc;

import java.io.IOException;
import org.apache.flink.api.common.serialization.DeserializationSchema;
import org.apache.flink.api.common.typeinfo.TypeInformation;
import trackline.v1.Events;

/**
 * Decodes TelemetryReceived into an {@link Observation}. A message that does
 * not decode is dropped (null) rather
 * than failing the job: one poison message must not stop the stream, and the
 * position-updater already dead-letters schema mismatches for the whole
 * pipeline.
 */
public class TelemetryDeserializer implements DeserializationSchema<Observation> {
    @Override
    public Observation deserialize(byte[] message) throws IOException {
        Events.TelemetryReceived ev;
        try {
            ev = Events.TelemetryReceived.parseFrom(message);
        } catch (com.google.protobuf.InvalidProtocolBufferException e) {
            return null;
        }
        if (!ev.hasPoint()) {
            return null;
        }
        // Only the device is known here; Enrich adds the vehicle, tenant and fleet.
        Observation o = new Observation();
        o.deviceId = ev.getDeviceId();
        o.recordedAtMs = ev.getPoint().getRecordedAt();
        o.ingestedAtMs = ev.getIngestedAt();
        o.lat = ev.getPoint().getPosition().getLatitude();
        o.lon = ev.getPoint().getPosition().getLongitude();
        o.speedKph = ev.getPoint().getSpeedKph();
        o.ignitionOn = ev.getPoint().getIgnitionOn();
        return o;
    }

    @Override
    public boolean isEndOfStream(Observation next) {
        return false;
    }

    @Override
    public TypeInformation<Observation> getProducedType() {
        return TypeInformation.of(Observation.class);
    }
}
