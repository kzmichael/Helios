package io.trackline.billing;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class BillingTest {
    static final BillingProperties.Plan PRO = new BillingProperties.Plan(499_900, 1_500, 40, 5_000_000);
    static final BillingProperties.Plan FREE = new BillingProperties.Plan(0, 0, 0, 0);

    @Test
    void proInvoiceHasPlatformFeeDevicesAndOverage() {
        // 30 days x 200 devices, 12,000,500 points: 7,000,500 beyond the 5,000,000
        // included.
        Pricing.Invoice inv = Pricing.price("pro", new Pricing.Usage(12_000_500, 6_000, 0), PRO, 1800);

        assertEquals(3, inv.lines().size());
        assertEquals(499_900, inv.lines().get(0).amountMinor());
        assertEquals(6_000L * 1_500, inv.lines().get(1).amountMinor());

        Pricing.Line overage = inv.lines().get(2);
        assertEquals(new BigDecimal("7000.500"), overage.quantity());
        assertEquals(280_020, overage.amountMinor()); // 7,000,500 * 40 / 1000 = 280,020 exactly

        long subtotal = 499_900 + 9_000_000 + 280_020;
        assertEquals(subtotal, inv.subtotalMinor());
        assertEquals(Pricing.tax(subtotal, 1800), inv.taxMinor());
        assertEquals(subtotal + inv.taxMinor(), inv.totalMinor());
    }

    @Test
    void linesAlwaysSumToTheSubtotal() {
        for (long points : new long[] { 0, 1, 999, 5_000_001, 77_777_777 }) {
            Pricing.Invoice inv = Pricing.price("pro", new Pricing.Usage(points, 31, 0), PRO, 1800);
            assertEquals(inv.lines().stream().mapToLong(Pricing.Line::amountMinor).sum(), inv.subtotalMinor());
            assertEquals(inv.subtotalMinor() + inv.taxMinor(), inv.totalMinor());
        }
    }

    @Test
    void fractionalAmountsRoundHalfUpToWholeMinorUnits() {
        // 1,501 points beyond the allowance at 40 per 1,000 = 60.04 -> 60.
        assertEquals(60, Pricing.price("pro", new Pricing.Usage(5_001_501, 0, 0), PRO, 0).lines().get(1).amountMinor());
        // 12,500 beyond at 40 per 1,000 = 500.0 exactly; 12,513 = 500.52 -> 501.
        assertEquals(501,
                Pricing.price("pro", new Pricing.Usage(5_012_513, 0, 0), PRO, 0).lines().get(1).amountMinor());
        // GST on 1,003 at 18% = 180.54 -> 181; on 1,002 = 180.36 -> 180.
        assertEquals(181, Pricing.tax(1_003, 1800));
        assertEquals(180, Pricing.tax(1_002, 1800));
    }

    @Test
    void aFreeTenantWithUsageStillOwesNothing() {
        assertTrue(Pricing.price("free", new Pricing.Usage(50_000, 300, 0), FREE, 1800).isZero());
    }

    @Test
    void usageInsideTheAllowanceHasNoOverageLine() {
        Pricing.Invoice inv = Pricing.price("pro", new Pricing.Usage(4_999_999, 0, 0), PRO, 1800);
        assertEquals(1, inv.lines().size());
    }

    @Test
    void anOverflowIsAnErrorNotAWrongInvoice() {
        assertThrows(ArithmeticException.class,
                () -> Pricing.price("pro", new Pricing.Usage(0, Long.MAX_VALUE / 2, 0), PRO, 1800));
    }

    @Test
    void periodsParseAndCloseAfterTheirLastDay() {
        BillingPeriod p = BillingPeriod.parse("2026-09");
        assertEquals(LocalDate.of(2026, 9, 1), p.start());
        assertEquals(LocalDate.of(2026, 9, 30), p.end());
        assertFalse(p.isClosed(LocalDate.of(2026, 9, 30)));
        assertTrue(p.isClosed(LocalDate.of(2026, 10, 1)));
        assertEquals("INV-202609-NORTHWIND", p.invoiceNumber("northwind"));
        assertEquals("INV-202609-ACME-CO", p.invoiceNumber("acme.co"));
        assertThrows(RuntimeException.class, () -> BillingPeriod.parse("September"));
    }

    private static BillingRepository.StoredInvoice stored(long lines, long subtotal, long tax, long total) {
        return new BillingRepository.StoredInvoice(UUID.randomUUID(), "INV-1", UUID.randomUUID(), "pro", "issued",
                subtotal, tax,
                total, lines);
    }

    @Test
    void aConsistentInvoiceReconciles() {
        Pricing.Invoice priced = Pricing.price("pro", new Pricing.Usage(6_000_000, 100, 0), PRO, 1800);
        var s = stored(priced.subtotalMinor(), priced.subtotalMinor(), priced.taxMinor(), priced.totalMinor());
        assertEquals(List.of(), Reconciliation.check(s, priced, 1800));
    }

    @Test
    void everyBrokenIdentityIsReported() {
        Pricing.Invoice priced = Pricing.price("pro", new Pricing.Usage(6_000_000, 100, 0), PRO, 1800);
        long sub = priced.subtotalMinor();

        assertEquals(1, Reconciliation.check(stored(sub - 1, sub, priced.taxMinor(), priced.totalMinor()), priced, 1800)
                .size());
        assertEquals(2, Reconciliation.check(stored(sub, sub, priced.taxMinor() + 1, priced.totalMinor()), priced, 1800)
                .size());
        assertEquals(1, Reconciliation.check(stored(sub, sub, priced.taxMinor(), priced.totalMinor() + 1), priced, 1800)
                .size());
    }

    @Test
    void usageThatChangedAfterBillingIsFlagged() {
        Pricing.Invoice billed = Pricing.price("pro", new Pricing.Usage(6_000_000, 100, 0), PRO, 1800);
        Pricing.Invoice now = Pricing.price("pro", new Pricing.Usage(6_000_000, 120, 0), PRO, 1800);
        var s = stored(billed.subtotalMinor(), billed.subtotalMinor(), billed.taxMinor(), billed.totalMinor());
        List<String> problems = Reconciliation.check(s, now, 1800);
        assertEquals(1, problems.size());
        assertTrue(problems.get(0).contains("usage changed after billing"));
    }

    @Test
    void databaseUrlBecomesJdbc() {
        String[] p = DataSourceConfig.toJdbc("postgres://u:p%40ss@db:6543/trackline?sslmode=disable");
        assertEquals("jdbc:postgresql://db:6543/trackline?sslmode=disable", p[0]);
        assertEquals("u", p[1]);
        assertEquals("p@ss", p[2]);
    }
}
