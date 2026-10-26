package io.trackline.billing;

import java.util.Map;
import org.springframework.boot.context.properties.ConfigurationProperties;

/** Pricing configuration. Everything is in integer minor units. */
@ConfigurationProperties("billing")
public record BillingProperties(String currency, int taxRateBps, Map<String, Plan> plans) {

    public record Plan(long baseMinor, long deviceDayMinor, long per1kPointsMinor, long includedPoints) {
    }

    public Plan planFor(String tier) {
        Plan plan = plans.get(tier);
        if (plan == null) {
            throw new IllegalStateException("no billing plan configured for tier '" + tier + "'");
        }
        return plan;
    }
}
