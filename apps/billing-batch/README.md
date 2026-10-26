# billing-batch

The nightly usage rollup (Java 17, Spring Boot 3.4, Spring Batch 5.2). It turns a
month of `usage_daily` rows into `invoice` and `invoice_line` rows, then
**reconciles the books and fails the job if anything is off**. Run it as a
Kubernetes CronJob, or by hand.

```bash
java -jar target/billing-batch.jar --period=2026-09     # a specific month (UTC)
java -jar target/billing-batch.jar                      # defaults to last month
BILLING_PERIOD=2026-09 java -jar target/billing-batch.jar
```

| Exit code | Meaning                                                                 |
| --------- | ----------------------------------------------------------------------- |
| `0`       | Invoices written **and** reconciled                                     |
| `2`       | The job failed, including a reconciliation failure — a person must look |
| `64`      | Bad `--period`                                                          |

## How it prices

Per tenant, from its tier's plan in `application.yml`:

| Line                                  | Quantity                                                  | Price                   |
| ------------------------------------- | --------------------------------------------------------- | ----------------------- |
| Platform fee                          | 1                                                         | `base-minor`            |
| Active device-days                    | sum of `usage_daily.active_devices`                       | `device-day-minor` each |
| Telemetry points beyond the allowance | (points − `included-points`) / 1,000, exact to 3 decimals | `per-1k-points-minor`   |

Then GST at `tax-rate-bps` (1800 = 18%). Everything is integer minor units (paise);
the only rounding is half-up, once per line and once for tax. A tenant that owes
nothing (the Free tier) gets no invoice. The prices in `application.yml` are
placeholders chosen to make the maths exercisable, not a commercial decision — change
them there, no code needed.

## Correctness, which is the point of this service

- **Idempotent reruns.** An invoice's identity is `(tenant, period)`. A rerun of a
  period creates nothing twice: a `draft` is recomputed and rewritten; an `issued`,
  `paid` or `void` invoice is never touched. Batch's own job metadata is deliberately
  not persisted (no batch tables are added to the database), so a rerun is safe
  because the writer is idempotent, not because Batch remembers.
- **One transaction per tenant.** A failure on one tenant leaves the others written
  and a rerun finishes the job.
- **Draft vs issued.** A period still in progress produces `draft` invoices that each
  run rewrites; once the period's last day is past they are written `issued`.
- **Reconciliation step.** After writing, every invoice for the period is checked:
  lines sum to the subtotal; tax equals the configured rate of the subtotal; subtotal
  plus tax equals the total; and the usage _now_ still prices to the invoiced subtotal
  (late-arriving usage after an invoice was issued is flagged, not silently
  absorbed). Any failure fails the job.
- **Audit trail.** Every invoice created or regenerated writes an `audit_log` row
  (`actor_kind=service`, `actor_id=billing-batch`, action `invoice.generated` /
  `invoice.regenerated`, with the period and total).

## Configuration

| Variable         | Meaning                                                                                            |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| `DATABASE_URL`   | `postgres://user:pass@host:port/db` (required); or `spring.datasource.*`                           |
| `BILLING_PERIOD` | Period when `--period` is not given; default is the previous month                                 |
| `billing.*`      | Pricing, via `application.yml` or Spring's usual environment overrides (e.g. `BILLING_TAXRATEBPS`) |

For a CronJob use `concurrencyPolicy: Forbid`: two runs of the same period at once
would contend on the same invoice rows and one would fail on the unique key.

## Build and test

```bash
cd apps/billing-batch && mvn package
```

Needs Java 17 (verified in `maven:3.9-eclipse-temurin-17`). The tests cover pricing,
rounding, overflow, period handling and every reconciliation identity.

## Verified

Against a throwaway Postgres with September usage inserted: the first run created and
issued two invoices and reconciled; the second left them unchanged (exit 0); the
current month produced drafts that were regenerated on rerun; deliberately corrupting
an invoice's tax made the run exit `2` with the failing invoice named; a bad
`--period` exited `64`.

## What's deliberately not implemented

- Payment, dunning, PDF/e-mail rendering of invoices, credit notes: nothing in the
  contracts asks for them.
- API request counts: `usage_daily.api_requests` exists but nothing writes it yet, so
  it is priced as zero.
