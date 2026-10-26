package io.trackline.billing;

import java.time.LocalDate;
import java.time.YearMonth;
import java.time.ZoneOffset;

/** The billing period: one calendar month, UTC, given as YYYY-MM. */
public record BillingPeriod(LocalDate start, LocalDate end) {

    public static BillingPeriod parse(String yyyyMm) {
        YearMonth ym = YearMonth.parse(yyyyMm);
        return new BillingPeriod(ym.atDay(1), ym.atEndOfMonth());
    }

    /** The month before the current one: what a nightly run on the 1st bills. */
    public static BillingPeriod previousMonth() {
        return parse(YearMonth.now(ZoneOffset.UTC).minusMonths(1).toString());
    }

    /**
     * A period is closed once its last day is in the past; only then is an invoice
     * issued.
     */
    public boolean isClosed(LocalDate today) {
        return end.isBefore(today);
    }

    public String label() {
        return YearMonth.from(start).toString();
    }

    /**
     * INV-202609-NORTHWIND: unique per tenant and period, and readable on a
     * statement.
     */
    public String invoiceNumber(String tenantSlug) {
        return "INV-" + label().replace("-", "") + "-" + tenantSlug.toUpperCase().replaceAll("[^A-Z0-9]+", "-");
    }
}
