package io.trackline.streamproc;

/**
 * One telemetry point, enriched with who it belongs to. Public fields and a no-arg
 * constructor make this a Flink POJO, so it is serialised efficiently between operators.
 */
public class Observation {
    public String vehicleId;
    public String deviceId;
    public String tenantId;
    public String fleetId;
    public String registration;
    public double speedLimitKph;

    public long recordedAtMs;
    public long ingestedAtMs;
    public double lat;
    public double lon;
    public double speedKph;
    public boolean ignitionOn;

    public Observation() {}
}
