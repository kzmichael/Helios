package io.trackline.billing;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * The accounting identities every invoice must satisfy. Pure functions over
 * stored
 * numbers, so they can be tested without a database and re-checked by anyone.
 */
public final class Reconciliation {
    private Reconciliation() {
    }

    /**
     * @param stored     what is in the database
     * @param recomputed what the same usage prices to now
     * @return human-readable problems; empty means the invoice reconciles
     */
    public static List<String> check(BillingRepository.StoredInvoice stored, Pricing.Invoice recomputed,
            int taxRateBps) {
        List<String> problems = new ArrayList<>();

        if (stored.linesMinor() != stored.subtotalMinor()) {
            problems.add(stored.number() + ": lines sum to " + stored.linesMinor() + " but subtotal is "
                    + stored.subtotalMinor());
        }
        long expectedTax = Pricing.tax(stored.subtotalMinor(), taxRateBps);
        if (stored.taxMinor() != expectedTax) {
            problems.add(stored.number() + ": tax is " + stored.taxMinor() + " but " + taxRateBps
                    + " bps of the subtotal is " + expectedTax);
        }
        if (stored.subtotalMinor() + stored.taxMinor() != stored.totalMinor()) {
            problems.add(stored.number() + ": subtotal + tax is " + (stored.subtotalMinor() + stored.taxMinor())
                    + " but total is "
                    + stored.totalMinor());
        }
        if (recomputed != null && recomputed.subtotalMinor() != stored.subtotalMinor()) {
            problems.add(stored.number() + ": usage now prices to " + recomputed.subtotalMinor() + " but the "
                    + stored.status()
                    + " invoice says " + stored.subtotalMinor() + " (usage changed after billing?)");
        }
        return problems;
    }

    public static String summary(Map<String, Integer> counts) {
        return "created=" + counts.getOrDefault("created", 0) + " regenerated=" + counts.getOrDefault("regenerated", 0)
                + " unchanged=" + counts.getOrDefault("unchanged", 0) + " skippedZero="
                + counts.getOrDefault("skippedZero", 0);
    }
}
