package io.trackline.streamproc;

import java.net.URI;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.function.Function;

/**
 * Runtime configuration, from the environment. No file ever contains a secret.
 */
public final class Config implements java.io.Serializable {
    public final String kafkaBrokers;
    public final String groupId;
    public final String jdbcUrl;
    public final String dbUser;
    public final String dbPassword;
    public final long registryTtlMs;
    public final long checkpointIntervalMs;
    public final String checkpointDir;
    public final int parallelism;
    public final boolean exactlyOnce;
    public final long usageWindowSeconds;
    public final String startingOffsets;

    public Config(Function<String, String> env) {
        this.kafkaBrokers = or(env.apply("KAFKA_BROKERS"), "kafka:9092");
        this.groupId = or(env.apply("KAFKA_GROUP_ID"), "stream-processor");

        String databaseUrl = env.apply("DATABASE_URL");
        if (databaseUrl == null || databaseUrl.isBlank()) {
            throw new IllegalStateException(
                    "DATABASE_URL must be set (the device and geofence registry lives in Postgres)");
        }
        String[] jdbc = toJdbc(databaseUrl);
        this.jdbcUrl = jdbc[0];
        this.dbUser = jdbc[1];
        this.dbPassword = jdbc[2];

        this.registryTtlMs = Long.parseLong(or(env.apply("REGISTRY_TTL_SECONDS"), "30")) * 1000;
        this.checkpointIntervalMs = Long.parseLong(or(env.apply("CHECKPOINT_INTERVAL_MS"), "30000"));
        this.checkpointDir = env.apply("CHECKPOINT_DIR");
        this.parallelism = Integer.parseInt(or(env.apply("PARALLELISM"), "1"));
        this.exactlyOnce = "true".equalsIgnoreCase(env.apply("EXACTLY_ONCE"));
        this.usageWindowSeconds = Long.parseLong(or(env.apply("USAGE_WINDOW_SECONDS"), "30"));
        this.startingOffsets = or(env.apply("STARTING_OFFSETS"), "latest");
    }

    public static Config fromEnv() {
        return new Config(System::getenv);
    }

    /**
     * postgres://user:pass@host:port/db?sslmode=disable -> {jdbc url, user,
     * password}.
     */
    static String[] toJdbc(String databaseUrl) {
        URI uri = URI.create(databaseUrl.replaceFirst("^postgres(ql)?://", "http://"));
        String user = "";
        String password = "";
        if (uri.getUserInfo() != null) {
            String[] parts = uri.getUserInfo().split(":", 2);
            user = URLDecoder.decode(parts[0], StandardCharsets.UTF_8);
            password = parts.length > 1 ? URLDecoder.decode(parts[1], StandardCharsets.UTF_8) : "";
        }
        int port = uri.getPort() > 0 ? uri.getPort() : 5432;
        StringBuilder url = new StringBuilder("jdbc:postgresql://").append(uri.getHost()).append(':').append(port)
                .append(uri.getPath());
        if (uri.getQuery() != null && !uri.getQuery().isBlank()) {
            url.append('?').append(uri.getQuery());
        }
        return new String[] { url.toString(), user, password };
    }

    private static String or(String v, String fallback) {
        return v == null || v.isBlank() ? fallback : v;
    }

    @Override
    public String toString() {
        // Deliberately omits the database password.
        return "Config{brokers=" + kafkaBrokers + ", group=" + groupId + ", db=" + jdbcUrl + ", parallelism="
                + parallelism
                + ", checkpointMs=" + checkpointIntervalMs + ", exactlyOnce=" + exactlyOnce + "}";
    }

    /** For tests. */
    public static Config of(Map<String, String> env) {
        return new Config(env::get);
    }
}
