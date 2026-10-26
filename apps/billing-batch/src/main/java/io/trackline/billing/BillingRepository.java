package io.trackline.billing;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.OffsetDateTime;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

/** All SQL for the job. Every statement is scoped by tenant and period. */
@Repository
public class BillingRepository {
    public record Tenant(UUID id, String slug, String tier) {
    }

    public record ExistingInvoice(UUID id, String status) {
    }

    /** An invoice as stored, with the sum of its lines, for reconciliation. */
    public record StoredInvoice(UUID id, String number, UUID tenantId, String tier, String status, long subtotalMinor,
            long taxMinor,
            long totalMinor, long linesMinor) {
    }

    private final JdbcTemplate jdbc;

    public BillingRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<Tenant> activeTenants() {
        return jdbc.query("SELECT id, slug, tier FROM tenant WHERE status = 'active' ORDER BY slug",
                (rs, i) -> new Tenant(rs.getObject("id", UUID.class), rs.getString("slug"), rs.getString("tier")));
    }

    public Pricing.Usage usage(UUID tenantId, LocalDate from, LocalDate to) {
        return jdbc.queryForObject(
                "SELECT coalesce(sum(telemetry_points), 0), coalesce(sum(active_devices), 0), coalesce(sum(api_requests), 0) "
                        + "FROM usage_daily WHERE tenant_id = ? AND usage_date BETWEEN ? AND ?",
                (rs, i) -> new Pricing.Usage(rs.getLong(1), rs.getLong(2), rs.getLong(3)), tenantId, from, to);
    }

    /**
     * Locks the tenant's invoice for the period, if there is one, so concurrent
     * runs queue up.
     */
    public Optional<ExistingInvoice> lockInvoice(UUID tenantId, LocalDate from, LocalDate to) {
        return jdbc.query(
                "SELECT id, status FROM invoice WHERE tenant_id = ? AND period_start = ? AND period_end = ? FOR UPDATE",
                (rs, i) -> new ExistingInvoice(rs.getObject("id", UUID.class), rs.getString("status")), tenantId, from,
                to)
                .stream().findFirst();
    }

    public UUID insertInvoice(UUID tenantId, String number, LocalDate from, LocalDate to, String currency,
            Pricing.Invoice inv,
            String status, OffsetDateTime issuedAt) {
        UUID id = UUID.randomUUID();
        jdbc.update(
                "INSERT INTO invoice (id, tenant_id, number, period_start, period_end, currency, subtotal_minor, tax_minor, "
                        + "total_minor, status, issued_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
                id, tenantId, number, from, to,
                currency, inv.subtotalMinor(), inv.taxMinor(), inv.totalMinor(), status, issuedAt);
        insertLines(id, inv);
        return id;
    }

    /** Only a draft may be rewritten; the caller has already checked. */
    public void replaceInvoice(UUID invoiceId, Pricing.Invoice inv, String status, OffsetDateTime issuedAt) {
        jdbc.update(
                "UPDATE invoice SET subtotal_minor = ?, tax_minor = ?, total_minor = ?, status = ?, issued_at = ? WHERE id = ?",
                inv.subtotalMinor(), inv.taxMinor(), inv.totalMinor(), status, issuedAt, invoiceId);
        jdbc.update("DELETE FROM invoice_line WHERE invoice_id = ?", invoiceId);
        insertLines(invoiceId, inv);
    }

    private void insertLines(UUID invoiceId, Pricing.Invoice inv) {
        short lineNo = 1;
        for (Pricing.Line l : inv.lines()) {
            jdbc.update(
                    "INSERT INTO invoice_line (id, invoice_id, description, quantity, unit_price_minor, amount_minor, line_no) "
                            + "VALUES (?, ?, ?, ?, ?, ?, ?)",
                    UUID.randomUUID(), invoiceId, l.description(), l.quantity(), l.unitPriceMinor(),
                    l.amountMinor(), lineNo++);
        }
    }

    public void audit(UUID tenantId, String action, UUID invoiceId, String metadataJson) {
        jdbc.update("INSERT INTO audit_log (tenant_id, actor_kind, actor_id, action, target_type, target_id, metadata) "
                + "VALUES (?, 'service', 'billing-batch', ?, 'invoice', ?, ?::jsonb)", tenantId, action,
                invoiceId.toString(),
                metadataJson);
    }

    public List<StoredInvoice> invoicesForPeriod(LocalDate from, LocalDate to) {
        return jdbc.query(
                "SELECT i.id, i.number, i.tenant_id, t.tier, i.status, i.subtotal_minor, i.tax_minor, i.total_minor, "
                        + "coalesce((SELECT sum(l.amount_minor) FROM invoice_line l WHERE l.invoice_id = i.id), 0) AS lines_minor "
                        + "FROM invoice i JOIN tenant t ON t.id = i.tenant_id "
                        + "WHERE i.period_start = ? AND i.period_end = ? ORDER BY i.number",
                (rs, n) -> new StoredInvoice(rs.getObject("id", UUID.class), rs.getString("number"),
                        rs.getObject("tenant_id", UUID.class), rs.getString("tier"), rs.getString("status"),
                        rs.getLong("subtotal_minor"), rs.getLong("tax_minor"), rs.getLong("total_minor"),
                        rs.getLong("lines_minor")),
                from, to);
    }

    /** Quantity is exposed for tests that read back what was written. */
    public List<BigDecimal> lineQuantities(UUID invoiceId) {
        return jdbc.queryForList("SELECT quantity FROM invoice_line WHERE invoice_id = ? ORDER BY line_no",
                BigDecimal.class,
                invoiceId);
    }
}
