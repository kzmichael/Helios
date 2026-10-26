package io.trackline.streamproc;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.Serializable;

/** A closed ring of [lon, lat] vertices, GeoJSON order. */
public final class Polygon implements Serializable {
    private static final ObjectMapper JSON = new ObjectMapper();

    private final double[][] ring;

    public Polygon(double[][] ring) {
        this.ring = ring;
    }

    /** Reads the outer ring of a GeoJSON Polygon. */
    public static Polygon fromGeoJson(String json) throws java.io.IOException {
        JsonNode root = JSON.readTree(json);
        if (!"Polygon".equals(root.path("type").asText()) || !root.path("coordinates").isArray()
                || root.path("coordinates").isEmpty()) {
            throw new IllegalArgumentException("not a polygon");
        }
        JsonNode outer = root.path("coordinates").get(0);
        double[][] ring = new double[outer.size()][2];
        for (int i = 0; i < outer.size(); i++) {
            ring[i][0] = outer.get(i).get(0).asDouble();
            ring[i][1] = outer.get(i).get(1).asDouble();
        }
        return new Polygon(ring);
    }

    /**
     * Ray casting. Points on the boundary may fall either way; GPS noise makes that
     * distinction meaningless.
     */
    public boolean contains(double lat, double lon) {
        int n = ring.length;
        if (n < 4) {
            return false;
        }
        boolean inside = false;
        for (int i = 0, j = n - 1; i < n; j = i++) {
            double xi = ring[i][0];
            double yi = ring[i][1];
            double xj = ring[j][0];
            double yj = ring[j][1];
            if ((yi > lat) != (yj > lat) && lon < (xj - xi) * (lat - yi) / (yj - yi) + xi) {
                inside = !inside;
            }
        }
        return inside;
    }
}
