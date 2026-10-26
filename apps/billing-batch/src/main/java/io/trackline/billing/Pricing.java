package io.trackline.billing;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.List;

/**
 * Turns a period's usage into invoice lines and totals. Pure arithmetic over
 * integers
 * and BigDecimal: no clock, no database, no floating point.
 */
public final class Pricing {
    private Pricing() {
    }

    /** What a tenant used in the period: sums over usage_daily. */
    public record Usage(long telemetryPoints, long activeDeviceDays, long apiRequests) {
    }

    public record Line(String description, BigDecimal quantity, long unitPriceMinor, long amountMinor) {
    }

    public record Invoice(List<Line> lines, long subtotalMinor, long taxMinor, long totalMinor) {
        public boolean isZero() {
            return totalMinor == 0;
        }
    }

    public static Invoice price(String tier, Usage usage, BillingProperties.Plan plan, int taxRateBps) {
        List<Line> lines = new ArrayList<>();

        if (plan.baseMinor() > 0) {
            lines.add(new Line("Platform fee (" + tier + ")", BigDecimal.ONE, plan.baseMinor(), plan.baseMinor()));
        }

        if (usage.activeDeviceDays() > 0 && plan.deviceDayMinor() > 0) {
            lines.add(
                    new Line("Active device-days", BigDecimal.valueOf(usage.activeDeviceDays()), plan.deviceDayMinor(),
                            Math.multiplyExact(usage.activeDeviceDays(), plan.deviceDayMinor())));
        }

        long billable = Math.max(0, usage.telemetryPoints() - plan.includedPoints());
        if (billable > 0 && plan.per1kPointsMinor() > 0) {
            // Quantity is in thousands of points, exact to three decimals; the amount is
            // rounded to a whole minor unit once, here, so a line is never off by a
            // fraction.
            BigDecimal thousands = BigDecimal.valueOf(billable).movePointLeft(3);
            long amount = BigDecimal.valueOf(billable).multiply(BigDecimal.valueOf(plan.per1kPointsMinor()))
                    .divide(BigDecimal.valueOf(1000), 0, RoundingMode.HALF_UP).longValueExact();
            lines.add(new Line("Telemetry points beyond " + plan.includedPoints() + " included (per 1,000)", thousands,
                    plan.per1kPointsMinor(), amount));
        }

        long subtotal = lines.stream().mapToLong(Line::amountMinor).sum();
        long tax = tax(subtotal, taxRateBps);
        return new Invoice(List.copyOf(lines), subtotal, tax, subtotal + tax);
    }

    public static long tax(long subtotalMinor, int taxRateBps) {
        return BigDecimal.valueOf(subtotalMinor).multiply(BigDecimal.valueOf(taxRateBps))
                .divide(BigDecimal.valueOf(10_000), 0, RoundingMode.HALF_UP).longValueExact();
    }
}
