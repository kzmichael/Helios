package io.trackline.streamproc;

import java.io.Serializable;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Who owns what: which vehicle and tenant a device belongs to, and which
 * geofences
 * apply. A read-through snapshot of Postgres, refreshed on a TTL and, at most
 * every
 * few seconds, on a miss (a device provisioned after the last refresh). If
 * Postgres
 * is unreachable the previous snapshot keeps being served.
 */
public final class Registry {
    private static final Logger LOG = LoggerFactory.getLogger(Registry.class);
    private static final long MISS_RETRY_MS = 5_000;

    public record Device(String deviceId, String vehicleId, String tenantId, String fleetId, String registration,
            double speedLimitKph)
            implements Serializable {
    }

    public record Geofence(String id, String tenantId, String fleetId, String name, String kind, int dwellSecs,
            Polygon boundary)
            implements Serializable {
    }

    private record Snapshot(Map<String, Device> byDevice, Map<String, List<Geofence>> geofencesByTenant) {
        static Snapshot empty() {
            return new Snapshot(Map.of(), Map.of());
        }
    }

    private static Registry instance;

    /**
     * One registry per task-manager JVM, shared by every parallel instance in it.
     */
    public static synchronized Registry get(Config cfg) {
        if (instance == null) {
            instance = new Registry(cfg);
        }
        return instance;
    }

    private final Config cfg;
    private volatile Snapshot snapshot = Snapshot.empty();
    private volatile long loadedAt;
    private volatile long lastMissTry;

    Registry(Config cfg) {
        this.cfg = cfg;
    }

    public Device byDevice(String deviceId) {
        maybeRefresh(false);
        Device d = snapshot.byDevice().get(deviceId);
        if (d == null) {
            maybeRefresh(true);
            d = snapshot.byDevice().get(deviceId);
        }
        return d;
    }

    /** The geofences that apply to a vehicle: its tenant's, narrowed by fleet. */
    public List<Geofence> geofencesFor(String tenantId, String fleetId) {
        maybeRefresh(false);
        List<Geofence> out = new ArrayList<>();
        for (Geofence g : snapshot.geofencesByTenant().getOrDefault(tenantId, Collections.emptyList())) {
            if (g.fleetId().isEmpty() || g.fleetId().equals(fleetId)) {
                out.add(g);
            }
        }
        return out;
    }

    private synchronized void maybeRefresh(boolean miss) {
        long now = System.currentTimeMillis();
        boolean stale = now - loadedAt > cfg.registryTtlMs;
        if (!stale) {
            if (!miss || now - lastMissTry < MISS_RETRY_MS) {
                return;
            }
            lastMissTry = now;
        }
        try {
            snapshot = load();
            loadedAt = now;
        } catch (SQLException e) {
            // Keep serving the previous snapshot; retry soon, but not on every element.
            loadedAt = now - cfg.registryTtlMs + MISS_RETRY_MS;
            LOG.warn("registry refresh failed, serving the previous snapshot: {}", e.getMessage());
        }
    }

    private Snapshot load() throws SQLException {
        Map<String, Device> devices = new HashMap<>();
        Map<String, List<Geofence>> fences = new HashMap<>();

        try (Connection c = DriverManager.getConnection(cfg.jdbcUrl, cfg.dbUser, cfg.dbPassword);
                Statement st = c.createStatement()) {
            try (ResultSet rs = st.executeQuery(
                    "SELECT d.id::text, v.id::text, v.tenant_id::text, v.fleet_id::text, v.registration, v.speed_limit_kph::float8 "
                            + "FROM device d JOIN vehicle v ON v.id = d.vehicle_id JOIN tenant t ON t.id = v.tenant_id "
                            + "WHERE d.status <> 'decommissioned' AND t.status = 'active'")) {
                while (rs.next()) {
                    Device d = new Device(rs.getString(1), rs.getString(2), rs.getString(3), rs.getString(4),
                            rs.getString(5), rs.getDouble(6));
                    devices.put(d.deviceId(), d);
                }
            }
            try (ResultSet rs = st.executeQuery(
                    "SELECT id::text, tenant_id::text, coalesce(fleet_id::text, ''), name, kind, dwell_threshold_s, boundary::text "
                            + "FROM geofence WHERE enabled")) {
                while (rs.next()) {
                    try {
                        Geofence g = new Geofence(rs.getString(1), rs.getString(2), rs.getString(3), rs.getString(4),
                                rs.getString(5),
                                rs.getInt(6), Polygon.fromGeoJson(rs.getString(7)));
                        fences.computeIfAbsent(g.tenantId(), k -> new ArrayList<>()).add(g);
                    } catch (Exception e) {
                        // A malformed boundary must not take the registry down.
                        LOG.warn("skipping geofence {}: {}", rs.getString(1), e.getMessage());
                    }
                }
            }
        }
        return new Snapshot(devices, fences);
    }
}
