package io.trackline.billing;

import java.time.LocalDate;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;
import javax.sql.DataSource;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.batch.core.Job;
import org.springframework.batch.core.Step;
import org.springframework.batch.core.configuration.annotation.StepScope;
import org.springframework.batch.core.job.builder.JobBuilder;
import org.springframework.batch.core.repository.JobRepository;
import org.springframework.batch.core.step.builder.StepBuilder;
import org.springframework.batch.item.ItemProcessor;
import org.springframework.batch.item.ItemWriter;
import org.springframework.batch.item.database.JdbcCursorItemReader;
import org.springframework.batch.item.database.builder.JdbcCursorItemReaderBuilder;
import org.springframework.batch.repeat.RepeatStatus;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.transaction.PlatformTransactionManager;

/**
 * usageToInvoices: step 1 prices every active tenant and writes its invoice
 * (one
 * transaction per tenant, so a failure leaves the others untouched and a rerun
 * repeats nothing); step 2 reconciles the books and fails the job if anything
 * is off.
 */
@Configuration
public class BatchConfig {
    private static final Logger LOG = LoggerFactory.getLogger(BatchConfig.class);

    /** Counters for the run summary. */
    @Bean
    RunStats runStats() {
        return new RunStats();
    }

    public static class RunStats {
        final Map<String, AtomicInteger> counts = new ConcurrentHashMap<>();

        void inc(String k) {
            counts.computeIfAbsent(k, x -> new AtomicInteger()).incrementAndGet();
        }

        Map<String, Integer> snapshot() {
            Map<String, Integer> m = new java.util.HashMap<>();
            counts.forEach((k, v) -> m.put(k, v.get()));
            return m;
        }
    }

    public record Draft(BillingRepository.Tenant tenant, Pricing.Invoice invoice) {
    }

    @Bean
    Job usageToInvoices(JobRepository repo, Step invoiceTenants, Step reconcile) {
        return new JobBuilder("usageToInvoices", repo).start(invoiceTenants).next(reconcile).build();
    }

    @Bean
    Step invoiceTenants(JobRepository repo, PlatformTransactionManager tx,
            JdbcCursorItemReader<BillingRepository.Tenant> tenantReader,
            ItemProcessor<BillingRepository.Tenant, Draft> pricer, ItemWriter<Draft> invoiceWriter) {
        return new StepBuilder("invoiceTenants", repo).<BillingRepository.Tenant, Draft>chunk(1, tx)
                .reader(tenantReader)
                .processor(pricer).writer(invoiceWriter).build();
    }

    @Bean
    Step reconcile(JobRepository repo, PlatformTransactionManager tx,
            org.springframework.batch.core.step.tasklet.Tasklet reconciler) {
        return new StepBuilder("reconcile", repo).tasklet(reconciler, tx).build();
    }

    @Bean
    JdbcCursorItemReader<BillingRepository.Tenant> tenantReader(DataSource ds) {
        return new JdbcCursorItemReaderBuilder<BillingRepository.Tenant>().name("tenantReader").dataSource(ds)
                .sql("SELECT id, slug, tier FROM tenant WHERE status = 'active' ORDER BY slug")
                .rowMapper((rs, i) -> new BillingRepository.Tenant(rs.getObject("id", java.util.UUID.class),
                        rs.getString("slug"),
                        rs.getString("tier")))
                .build();
    }

    @Bean
    @StepScope
    ItemProcessor<BillingRepository.Tenant, Draft> pricer(BillingRepository repo, BillingProperties props,
            @Value("#{jobParameters['period']}") String period) {
        BillingPeriod p = BillingPeriod.parse(period);
        return tenant -> {
            Pricing.Usage usage = repo.usage(tenant.id(), p.start(), p.end());
            Pricing.Invoice inv = Pricing.price(tenant.tier(), usage, props.planFor(tenant.tier()), props.taxRateBps());
            return new Draft(tenant, inv);
        };
    }

    @Bean
    @StepScope
    ItemWriter<Draft> invoiceWriter(BillingRepository repo, BillingProperties props, RunStats stats,
            @Value("#{jobParameters['period']}") String period) {
        BillingPeriod p = BillingPeriod.parse(period);
        return chunk -> {
            for (Draft d : chunk) {
                write(repo, props, stats, p, d);
            }
        };
    }

    static void write(BillingRepository repo, BillingProperties props, RunStats stats, BillingPeriod p, Draft d) {
        LocalDate today = LocalDate.now(ZoneOffset.UTC);
        boolean closed = p.isClosed(today);
        String status = closed ? "issued" : "draft";
        OffsetDateTime issuedAt = closed ? OffsetDateTime.now(ZoneOffset.UTC) : null;

        var existing = repo.lockInvoice(d.tenant().id(), p.start(), p.end());
        String meta = "{\"period\":\"" + p.label() + "\",\"totalMinor\":" + d.invoice().totalMinor() + "}";

        if (existing.isEmpty()) {
            if (d.invoice().isZero()) {
                stats.inc("skippedZero");
                LOG.info("{}: nothing to bill for {}", d.tenant().slug(), p.label());
                return;
            }
            var id = repo.insertInvoice(d.tenant().id(), p.invoiceNumber(d.tenant().slug()), p.start(), p.end(),
                    props.currency(),
                    d.invoice(), status, issuedAt);
            repo.audit(d.tenant().id(), "invoice.generated", id, meta);
            stats.inc("created");
            LOG.info("{}: {} invoice {} total {} {}", d.tenant().slug(), status, p.invoiceNumber(d.tenant().slug()),
                    d.invoice().totalMinor(), props.currency());
        } else if ("draft".equals(existing.get().status())) {
            repo.replaceInvoice(existing.get().id(), d.invoice(), status, issuedAt);
            repo.audit(d.tenant().id(), "invoice.regenerated", existing.get().id(), meta);
            stats.inc("regenerated");
            LOG.info("{}: draft invoice rewritten as {}", d.tenant().slug(), status);
        } else {
            // Issued, paid or void invoices are never rewritten; reconcile will say if
            // usage has moved.
            stats.inc("unchanged");
            LOG.info("{}: invoice already {}, left as it is", d.tenant().slug(), existing.get().status());
        }
    }

    @Bean
    @StepScope
    org.springframework.batch.core.step.tasklet.Tasklet reconciler(BillingRepository repo, BillingProperties props,
            RunStats stats,
            @Value("#{jobParameters['period']}") String period) {
        BillingPeriod p = BillingPeriod.parse(period);
        return (contribution, chunkContext) -> {
            List<String> problems = new ArrayList<>();
            var tiers = repo.activeTenants();
            for (var inv : repo.invoicesForPeriod(p.start(), p.end())) {
                var tenant = tiers.stream().filter(t -> t.id().equals(inv.tenantId())).findFirst().orElse(null);
                Pricing.Invoice recomputed = null;
                if (tenant != null) {
                    recomputed = Pricing.price(tenant.tier(), repo.usage(tenant.id(), p.start(), p.end()),
                            props.planFor(tenant.tier()),
                            props.taxRateBps());
                }
                problems.addAll(Reconciliation.check(inv, recomputed, props.taxRateBps()));
            }
            LOG.info("period {}: {}; reconciliation problems: {}", p.label(), Reconciliation.summary(stats.snapshot()),
                    problems.size());
            if (!problems.isEmpty()) {
                problems.forEach(m -> LOG.error("RECONCILIATION FAILED: {}", m));
                throw new IllegalStateException(problems.size() + " invoice(s) do not reconcile for " + p.label());
            }
            return RepeatStatus.FINISHED;
        };
    }
}
