package io.trackline.billing;

import java.net.URI;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import javax.sql.DataSource;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.PlatformTransactionManager;

/**
 * The database connection, from DATABASE_URL
 * (postgres://user:pass@host:port/db) like
 * every other Trackline service, or from spring.datasource.* if that is what a
 * deployment prefers.
 */
@Configuration
@EnableConfigurationProperties(BillingProperties.class)
public class DataSourceConfig {

    @Bean
    DataSource dataSource(Environment env) {
        String databaseUrl = env.getProperty("DATABASE_URL");
        if (databaseUrl == null || databaseUrl.isBlank()) {
            String jdbc = env.getProperty("spring.datasource.url");
            if (jdbc == null) {
                throw new IllegalStateException("DATABASE_URL must be set");
            }
            return DataSourceBuilder.create().url(jdbc).username(env.getProperty("spring.datasource.username"))
                    .password(env.getProperty("spring.datasource.password")).build();
        }
        String[] parts = toJdbc(databaseUrl);
        return DataSourceBuilder.create().url(parts[0]).username(parts[1]).password(parts[2]).build();
    }

    @Bean
    PlatformTransactionManager transactionManager(DataSource dataSource) {
        return new DataSourceTransactionManager(dataSource);
    }

    @Bean
    JdbcTemplate jdbcTemplate(DataSource dataSource) {
        return new JdbcTemplate(dataSource);
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
            String[] up = uri.getUserInfo().split(":", 2);
            user = URLDecoder.decode(up[0], StandardCharsets.UTF_8);
            password = up.length > 1 ? URLDecoder.decode(up[1], StandardCharsets.UTF_8) : "";
        }
        int port = uri.getPort() > 0 ? uri.getPort() : 5432;
        String url = "jdbc:postgresql://" + uri.getHost() + ":" + port + uri.getPath()
                + (uri.getQuery() != null && !uri.getQuery().isBlank() ? "?" + uri.getQuery() : "");
        return new String[] { url, user, password };
    }
}
