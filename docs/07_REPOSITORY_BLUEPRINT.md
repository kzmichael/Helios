# The Full Blueprint — Every File, Folder, and What It's For

> Companion to `START_HERE.md`. That file explains the *what* and *why* in prose. This
> file is the **map of the territory** — every directory and file this project will
> eventually contain, what goes in it, and a short reason it exists. No teaching here;
> just "this is what's here and why it's here."
>
> Read it once for the shape. Then use it as a reference — when a roadmap step says
> "build the Terraform module," come back here to see exactly what files that produces.
>
> **Phase tags** (`P0`, `P1`, …) show which roadmap phase first creates each item. Nothing
> before its phase tag should exist yet — if you find yourself wanting to create a `P5`
> file while still in `P1`, that's premature and the reason to stop.

---

## How to read this document

Each section is one top-level directory. Inside, a table: **path**, **phase it appears
in**, **what it is**, **why it exists**. Paths are relative to the repository root
(`helios/`, currently `Major Project/` in this workspace — renamed when the public repo
is created per `HEL-0.1.1`).

Generated files (compiled protobuf code, `node_modules`, Terraform providers, lockfile
binaries) are **never listed here** unless the fact that they're generated is itself the
point. This document is about what a human writes, not what a tool produces.

---

## 0. Repository root

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `README.md` | P0 | Entry point, doc index, repo layout, status table | First thing anyone reads, human or automated |
| `START_HERE.md` | P0 | Plain-language explainer + action key | The onboarding ramp; this document's companion |
| `LICENSE` | P0 | Open source license (MIT or Apache-2.0) | Public repo needs one or GitHub shows "no license," which reads as "all rights reserved" |
| `.gitignore` | P0 | Excludes state files, `.env`, secrets, build artifacts | The single most important file for a public repo |
| `.gitattributes` | P0 | Line-ending normalisation, marks generated files as `linguist-generated` | Keeps diffs clean; keeps GitHub's language stats honest |
| `.mise.toml` | P0 | Pinned exact versions of every tool (Go, Node, Rust, Python, kubectl, helm, terraform, k6, gitleaks) | Identical toolchain locally and in CI — eliminates "works on my machine" |
| `.editorconfig` | P0 | Indentation/charset rules across editors | Trivial, but prevents mixed-whitespace diffs in a polyglot repo |
| `Taskfile.yml` | P0 | The single entrypoint: `task up`, `task down`, `task nuke`, `task test`, `task seed`, `task load` | One command to remember instead of forty. Story `HEL-0.6.2`. |
| `docker-compose.yml` | P0 | Full local stack — Postgres, Redis, Kafka, all six services, device-sim | What `task up` actually runs in Phase 0 |
| `docker-compose.obs.yml` | P0 | Optional override adding Prometheus/Grafana/Loki/Tempo/OTel Collector | Split out so the "slim" profile (see `04_ENVIRONMENTS.md` §6) can omit it on 16 GB RAM |
| `Makefile` | — | *Not used* — superseded by `Taskfile.yml` | Recorded here only to say: pick one, don't maintain both |
| `CODEOWNERS` | P7 | Maps paths to an owner | Needed once Backstage's service catalog cares who owns what |
| `SECURITY.md` | P4 | How to report a vulnerability, supported versions | Expected on a public repo once it's a "real" project |

---

## 1. `docs/` — the design record

Everything here already exists except where marked. This is the layer above code:
decisions, plans, and study material.

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `docs/00_CHARTER.md` | P0 | Purpose, scope, principles, success criteria | Prevents scope creep and tool-collecting |
| `docs/01_APPLICATION.md` | P0 | Trackline architecture, services, data layer, SLO candidates | Defines the workload before any infra is justified |
| `docs/02_TECH_MATRIX.md` | P0 | Every technology, target depth, Phase 1 vs Phase 2 | Stops you going too deep on one tool too early |
| `docs/03_ROADMAP.md` | P0 | Phases, exit criteria, deliberate failures | The ordering contract |
| `docs/04_ENVIRONMENTS.md` | P0 | Cloud accounts, budget, cost model, cost controls | Money discipline, decided before spending starts |
| `docs/05_STUDY_GUIDE.md` | P0 | What to study per phase, with self-check questions | Turns "learn Kubernetes" into a bounded, checkable list |
| `docs/06_WAYS_OF_WORKING.md` | P0 | Work breakdown, story format, board mechanics | How the backlog and board relate |
| `docs/07_REPOSITORY_BLUEPRINT.md` | P0 | **This file** | The map of every file this project will contain |
| `docs/adr/000N-*.md` | ongoing | One file per Architecture Decision Record | The reasoning survives even when the author forgets it |
| `docs/diagrams/architecture.mmd` | P0 | Mermaid component diagram | Text-based, diffable, reviewed in PRs — not a drawing-tool file that rots |
| `docs/diagrams/request-flow.mmd` | P0 | Sequence diagram: device → map | The picture version of `START_HERE.md` §1.7 |
| `docs/diagrams/network-topology.mmd` | P1 | CIDR ranges, subnets, peering across environments | Prevents the overlapping-address-space mistake before it's expensive to fix |

---

## 2. `backlog/` — work as data

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `backlog/phase-0.yaml` … `phase-8.yaml` | rolling | One file per phase, epics + stories + spikes | Source of truth for the board; only one phase ahead is ever groomed |
| `backlog/schema.json` | P0 | JSON Schema validating the YAML shape | Lets `HEL-0.6.1`'s sync script fail loudly on a malformed story instead of silently |

---

## 3. `apps/` — application source (delivered, not built by the operator)

Already partially delivered. Full picture below so you can see what each pending service
will actually contain.

### 3.1 Shared contracts (delivered)

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/README.md` | P0 | Source overview, delivery status table | Orientation for this directory specifically |
| `apps/_contracts/openapi.yaml` | P0 | Public REST API, OpenAPI 3.1 | The only contract `web` and outside integrators may rely on |
| `apps/_contracts/auth.md` | P0 | Identity model, JWT validation, header-stripping rule | The security contract every service must satisfy |
| `apps/_contracts/limits.md` | P0 | Quotas, rate limits, load shedding order | The fairness/survival contract |
| `apps/_contracts/kafka-topics.md` | P0 | Topic names, keys, retention, DLQ policy | The event-backbone contract |
| `apps/_proto/buf.yaml` | P0 | Buf lint/breaking-change config | Enforces schema discipline mechanically |
| `apps/_proto/buf.gen.yaml` | P0 | Codegen targets (Go, TS) | One command regenerates both sides after a contract change |
| `apps/_proto/trackline/v1/common.proto` | P0 | Shared types: `GeoPoint`, `Tier`, `EventMeta` | Avoids duplicating the trace/idempotency envelope everywhere |
| `apps/_proto/trackline/v1/telemetry.proto` | P0 | `TelemetryIngest` gRPC service | Device ↔ ingest contract |
| `apps/_proto/trackline/v1/events.proto` | P0 | 9 Kafka event message types | Producer ↔ consumer contract |

### 3.2 Database (delivered)

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/_db/migrations/0001_init.sql` | P0 | 20-table schema, forward-only | The system of record's shape |
| `apps/_db/migrations/000N_*.sql` | rolling | Each subsequent schema change | Never edit `0001`; add a new numbered file |
| `apps/_db/seed/0001_tenants.sql` | P0 | 3 tenants, 7 fleets, 150 vehicles, deterministic | Reproducible non-empty starting state |

### 3.3 `edge-gateway/` (Go) — pending

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/edge-gateway/go.mod`, `go.sum` | P0 | Module definition and locked dependencies | Reproducible builds |
| `apps/edge-gateway/cmd/gateway/main.go` | P0 | Process entrypoint | Wires config, router, and graceful shutdown |
| `apps/edge-gateway/internal/auth/jwt.go` | P0 | JWT validation per `_contracts/auth.md` | The one place tokens are checked |
| `apps/edge-gateway/internal/auth/context.go` | P0 | Sets/strips `X-Trackline-*` headers | Implements the header-stripping rule — the confused-deputy fix |
| `apps/edge-gateway/internal/ratelimit/bucket.go` | P0 | Token bucket against Redis, fail-open | Implements `_contracts/limits.md` §4 |
| `apps/edge-gateway/internal/proxy/router.go` | P0 | Routes to `fleet-api`, `ingest` | The actual "gateway" behaviour |
| `apps/edge-gateway/internal/telemetry/otel.go` | P3 | OpenTelemetry instrumentation | Added when Phase 3 requires tracing, not before |
| `apps/edge-gateway/Dockerfile` | P0 | Multi-stage, distroless, non-root, multi-arch | Story `HEL-0.3.1` — the reference image |
| `apps/edge-gateway/README.md` | P0 | Ports, env vars, local run instructions | Orientation without reading the code |

### 3.4 `ingest/` (Go) — pending

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/ingest/cmd/ingest/main.go` | P0 | Entrypoint | — |
| `apps/ingest/internal/grpcserver/telemetry.go` | P0 | Implements `TelemetryIngest` service | The hot path |
| `apps/ingest/internal/mqtt/broker_client.go` | P2 | MQTT fallback transport | Constrained devices that can't hold a gRPC stream |
| `apps/ingest/internal/kafka/producer.go` | P0 | Idempotent, `acks=all` producer | Implements `_contracts/kafka-topics.md` §3 |
| `apps/ingest/internal/backpressure/shedder.go` | P6 | Implements shed-order from `limits.md` §6 | Only meaningfully testable once load tests exist |
| `apps/ingest/Dockerfile` | P0 | Same standard as `edge-gateway` | — |

### 3.5 `fleet-api/` (TypeScript/NestJS) — pending

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/fleet-api/package.json`, `pnpm-lock.yaml` | P0 | Dependencies, locked | — |
| `apps/fleet-api/src/main.ts` | P0 | Nest bootstrap | — |
| `apps/fleet-api/src/tenants/` | P0 | Tenant CRUD, quota checks | Implements `limits.md` §3 device-count enforcement |
| `apps/fleet-api/src/fleets/`, `vehicles/`, `drivers/` | P0 | Core CRUD modules | The boring-but-essential API surface |
| `apps/fleet-api/src/geofences/` | P0 | Geofence CRUD + validation | Validates GeoJSON polygons on write |
| `apps/fleet-api/src/alerts/` | P0 | Alert list/acknowledge endpoints | Backs the dashboard's alert feed |
| `apps/fleet-api/src/billing/` | P2 | Usage and invoice read endpoints | Depends on `billing-batch` existing |
| `apps/fleet-api/src/common/tenant-guard.ts` | P0 | Enforces 404-not-403 tenant isolation | Implements `auth.md` §4 |
| `apps/fleet-api/src/common/db.module.ts` | P0 | Postgres pool setup | Where pool-exhaustion bugs will surface |
| `apps/fleet-api/Dockerfile` | P0 | Node multi-stage, standalone output | — |

### 3.6 `notifier/` (Rust) — pending

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/notifier/Cargo.toml`, `Cargo.lock` | P0 | Dependencies, locked | — |
| `apps/notifier/src/main.rs` | P0 | Entrypoint, Kafka consumer loop | — |
| `apps/notifier/src/channels/email.rs` | P0 | SMTP to local Mailpit sink | Never reaches a real inbox in dev |
| `apps/notifier/src/channels/webhook.rs` | P0 | HMAC-signed outbound webhooks | Implements `auth.md` §7 |
| `apps/notifier/src/channels/sms.rs` | P2 | SMS provider sandbox integration | Deferred — real provider needed |
| `apps/notifier/src/retry.rs` | P0 | Exponential backoff with jitter, idempotency key check | The retry-storm lesson lives here |
| `apps/notifier/src/dlq.rs` | P0 | Publishes to `notifications.dlq` after N attempts | Implements `kafka-topics.md` §5 |
| `apps/notifier/Dockerfile` | P0 | Rust multi-stage, musl static binary | Smallest image of the six services |

### 3.7 `device-sim/` (Go) — pending

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/device-sim/cmd/simulator/main.go` | P0 | Entrypoint, reads a fleet profile | — |
| `apps/device-sim/internal/profiles/*.yaml` | P0 | Behaviour presets: commuter, long-haul, idle, erratic | Realistic, varied load instead of identical robots |
| `apps/device-sim/internal/chaos/` | P6 | Inject clock skew, packet loss, disconnects | Becomes the chaos driver in Phase 6 |
| `apps/device-sim/Dockerfile` | P0 | Standard Go image | — |

### 3.8 `web/` (Next.js) — pending

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/web/package.json` | P0 | Dependencies | — |
| `apps/web/app/dashboard/page.tsx` | P0 | Tenant dashboard shell | — |
| `apps/web/app/map/page.tsx` | P0 | Live map, WebSocket-driven | The visible payoff of the freshness SLO |
| `apps/web/app/alerts/page.tsx` | P0 | Alert list + acknowledge | — |
| `apps/web/lib/api-client.ts` | P0 | Generated from `openapi.yaml` | Never hand-write a client against a spec you own |
| `apps/web/lib/ws-client.ts` | P0 | WebSocket subscription to position updates | — |
| `apps/web/Dockerfile` | P0 | Next standalone output, multi-stage | — |

### 3.9 Phase 2 services — pending, later

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `apps/stream-proc/` (Java/Flink) | P2 | Windowed aggregation, geofence transitions, speeding/idle detection | Stateful streaming — deliberately not Phase 0 |
| `apps/eta-ml/` (Python/FastAPI) | P2 | ETA prediction, anomaly scoring, batch retraining | Needs GPU/CPU node pools that don't exist yet |
| `apps/billing-batch/` (Java/Spring Batch) | P2 | Nightly usage rollup → invoices | Correctness-first discipline, different from the rest |

---

## 4. `infra/` — Infrastructure as Code (built by the operator)

Nothing here exists yet. This is the largest section because it's the largest part of the
actual learning.

### 4.1 Terraform / OpenTofu

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `infra/terraform/modules/network/` | P1 | VPC/VNet, subnets, NAT, CIDR allocation | Reusable across every environment; CIDR planning matters once, here |
| `infra/terraform/modules/k8s-cluster/` | P1 | AKS module — node pools, spot config | Parameterised so `dev`/`stage`/`prod` differ only in tfvars |
| `infra/terraform/modules/postgres/` | P1 | Managed Postgres (prod) or CloudNativePG values (dev/stage) | Cost lever from `04_ENVIRONMENTS.md` §10.3 |
| `infra/terraform/modules/dns-cert/` | P1 | DNS zone records, cert-manager ClusterIssuer wiring | Ties into the domain bought at Phase 1 |
| `infra/terraform/modules/eks-cluster/` | P5 | AWS EKS — the ephemeral lab cluster | Destroyed nightly by design |
| `infra/terraform/modules/gke-cluster/` | P5 | GCP GKE Autopilot — DR target | Created only after the GCP account exists |
| `infra/terraform/live/dev/` | P1 | Backend config + tfvars for `dev` | One live dir per environment, thin, calls modules |
| `infra/terraform/live/stage/` | P1 | Same, for `stage` | — |
| `infra/terraform/live/prod/` | P1 | Same, for `prod` — the always-on tier | — |
| `infra/terraform/live/aws-lab/` | P5 | Ephemeral AWS environment | Nightly destroy target |
| `infra/terraform/live/gcp-dr/` | P5 | GCP DR environment | Phase 5 only |
| `infra/terraform/backend.tf` (per live dir) | P1 | Remote state config — encrypted, locked, private bucket | Never in the public repo's default state; backend only |
| `infra/terraform/versions.tf` (per module) | P1 | Provider version pins | Reproducible plans |

### 4.2 Bicep (second implementation, for comparison)

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `infra/bicep/modules/aks.bicep` | P5 | Same AKS cluster, written in Bicep | Direct comparison exercise — `ADR-0012` |
| `infra/bicep/main.bicep` | P5 | Composition entrypoint | — |
| `infra/bicep/main.bicepparam` | P5 | Per-environment parameters | — |

### 4.3 Pulumi

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `infra/pulumi/cloudflare/index.ts` | P5 | DNS, WAF rules, Tunnel config, one Worker | A third IaC tool, scoped to one layer only |
| `infra/pulumi/Pulumi.yaml` | P5 | Project manifest | — |

### 4.4 Crossplane (Phase 7 self-service layer)

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `infra/crossplane/compositions/postgres-database.yaml` | P7 | Composition a developer can claim via PR | The IDP's actual self-service mechanism |
| `infra/crossplane/definitions/xrd-database.yaml` | P7 | The claim's schema | What a developer is allowed to ask for |

### 4.5 Ansible (on-prem tier)

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `infra/ansible/inventories/onprem.ini` | P5 | Host list for the Mac-as-datacenter tier | — |
| `infra/ansible/roles/k3s-node/tasks/main.yml` | P5 | Installs and configures k3s | The actual config-management exercise |
| `infra/ansible/roles/cloudflared-tunnel/tasks/main.yml` | P5 | Installs and registers the Tunnel daemon | Makes the on-prem tier internet-reachable |
| `infra/ansible/playbooks/site.yml` | P5 | Top-level playbook tying roles together | Single entrypoint: `ansible-playbook site.yml` |
| `infra/ansible/molecule/k3s-node/molecule.yml` | P5 | Molecule test config for the role | Proves the role is idempotent, mechanically |

---

## 5. `deploy/` — how workloads reach a cluster

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `deploy/kustomize/base/*.yaml` | P1 | Base Deployment/Service/ConfigMap per app | Shared manifest, environment-agnostic |
| `deploy/kustomize/overlays/dev/kustomization.yaml` | P1 | Dev-specific patches (replica count, resource limits) | — |
| `deploy/kustomize/overlays/stage/kustomization.yaml` | P1 | Stage-specific patches | — |
| `deploy/kustomize/overlays/prod/kustomization.yaml` | P1 | Prod-specific patches (spot tolerations, PDBs) | — |
| `deploy/kustomize/overlays/preview/kustomization.yaml` | P2 | Template for ephemeral PR environments | Parameterised by PR number |
| `deploy/helm/fleet-api/Chart.yaml` | P2 | One service done in Helm, for comparison | `ADR-0006: Helm vs Kustomize` needs both to exist |
| `deploy/helm/fleet-api/values.yaml`, `values-prod.yaml` | P2 | Per-environment values | — |
| `deploy/argocd/app-of-apps.yaml` | P2 | Root Argo CD Application | Everything else is reconciled from this one object |
| `deploy/argocd/applications/*.yaml` | P2 | One Argo `Application` per service | Sync waves, health checks |
| `deploy/argocd/applicationsets/preview-envs.yaml` | P2 | Generates one Application per open PR | The preview-environment mechanism |
| `deploy/policies/kyverno/require-signed-images.yaml` | P4 | Admission policy rejecting unsigned images | The rule that must be demonstrably enforced |
| `deploy/policies/kyverno/disallow-root.yaml` | P4 | Admission policy rejecting root containers | — |
| `deploy/policies/kyverno/require-resource-limits.yaml` | P1 | Rejects pods with no CPU/memory limits | Prevents one pod starving its node |
| `deploy/policies/network/default-deny.yaml` | P4 | Baseline NetworkPolicy, deny-all | The floor everything else is layered on |
| `deploy/policies/network/allow-*.yaml` | P4 | Explicit allow rules per service pair | Documents the real traffic graph |
| `deploy/rollouts/fleet-api-rollout.yaml` | P2 | Argo Rollouts canary spec with analysis template | Automated rollback driven by an SLO query |

---

## 6. `observability/` — the five signals, as code

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `observability/otel/collector-config.yaml` | P3 | Receivers → processors → exporters pipeline | The single instrumentation point that fans out to N backends |
| `observability/otel/instrumentation/*.md` | P3 | Per-language instrumentation notes | Go/TS/Rust/Java/Python each wire OTel slightly differently |
| `observability/dashboards/red-fleet-api.json` | P3 | Grafana dashboard as code (provisioned, not clicked) | Rate/Errors/Duration for the primary API |
| `observability/dashboards/kafka-lag.json` | P3 | Consumer lag per group | The scaling signal, visualised |
| `observability/dashboards/cost-per-tenant.json` | P6 | FinOps dashboard | The payoff of `limits.md`'s tier-labelled metrics |
| `observability/alerts/burn-rate.yaml` | P3 | Multi-window multi-burn-rate Prometheus rules | The alerting mechanism the SLOs depend on |
| `observability/alerts/kafka.yaml` | P3 | Consumer lag, DLQ depth alerts | — |
| `observability/slo/fleet-api-availability.yaml` | P3 | SLO definition (Sloth/Pyrra format) | SLI #1 from `01_APPLICATION.md` §5, as code |
| `observability/slo/position-freshness.yaml` | P3 | The cross-service freshness SLO | SLI #4 — the hard one |
| `observability/slo/*.yaml` (remaining 5 SLIs) | P3 | One file per SLO candidate | — |

---

## 7. `security/` — supply chain and policy inputs

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `security/trivy/trivy.yaml` | P1 | Scan config, severity gate, ignore list with expiry | CI fails the build on unpatched criticals |
| `security/sbom/README.md` | P4 | How SBOMs are generated and attached (cosign attest) | Explains the mechanism the policy in `deploy/policies` enforces |
| `security/gitleaks/gitleaks.toml` | P0 | Custom secret-pattern rules | Tuned beyond the defaults for this repo's own token formats |
| `security/checkov/checkov.yaml` | P1 | IaC misconfiguration scan config | Gate on `infra/terraform` changes |
| `security/threat-models/edge-gateway-stride.md` | P4 | STRIDE threat model | Required exercise, one real service |

---

## 8. `platform/` — the internal developer platform (Phase 7)

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `platform/backstage/catalog-info.yaml` (per service) | P7 | Service catalog entry: owner, links, SLOs | Makes "who owns this" answerable in one click |
| `platform/backstage/templates/new-service/` | P7 | Scaffolder template | The golden path — one PR provisions CI/CD/o11y/policy |
| `platform/scorecards/rules.yaml` | P7 | What "good" means per service (o11y coverage, SLO presence, freshness) | Makes quality comparable across services |

---

## 9. `ops/` — runbooks, postmortems, disaster recovery

Grows continuously from Phase 0 onward; nothing here is ever deleted.

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `ops/runbooks/<alert-name>.md` | P3+ | One file per alert: symptom, diagnosis steps, fix | An alert with no runbook is a 3am puzzle |
| `ops/postmortems/phase-N-retro.md` | every phase | What broke, what changed | Mandatory closing artifact |
| `ops/postmortems/incident-YYYY-MM-DD-<slug>.md` | as needed | Blameless incident writeup with action items | The portfolio artifact this whole project produces |
| `ops/gamedays/gameday-001-<scenario>.md` | P6 | Hypothesis, blast radius, abort conditions, results | Chaos experiments are planned, not improvised |
| `ops/dr/restore-drill-log.md` | P6 | Timed record of an actual backup restore | "A backup never restored is not a backup" |
| `ops/dr/failover-drill-log.md` | P5 | Timed record of a real region failover | Proof the RTO/RPO in `01_APPLICATION.md` is real, not aspirational |

---

## 10. `tests/` — load, chaos, and contracts

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `tests/load/k6/smoke.js` | P1 | Minimal "does it respond" check | Fast CI gate |
| `tests/load/k6/soak.js` | P6 | Sustained load over hours | Finds slow leaks single requests never show |
| `tests/load/k6/spike.js` | P6 | Sudden traffic surge | Tests autoscaling reaction time |
| `tests/load/k6/breakpoint.js` | P6 | Ramps until failure | Finds the actual capacity ceiling |
| `tests/chaos/pod-kill.yaml` | P6 | Chaos Mesh experiment definition | Written hypothesis lives in the matching gameday doc |
| `tests/chaos/network-latency.yaml` | P6 | Injects latency between two services | Tests timeout/circuit-breaker behaviour |
| `tests/contract/fleet-api.pact.json` | P2 | Consumer-driven contract test | Catches `web`/`fleet-api` drift before deploy |

---

## 11. `.github/` — automation

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `.github/workflows/ci.yaml` | P0 | Build, unit test, lint — per service, path-filtered | Runs on every PR |
| `.github/workflows/image-scan.yaml` | P1 | Trivy + gitleaks + Checkov as required checks | Real security control, not just the pre-commit hook |
| `.github/workflows/build-push.yaml` | P1 | Build, sign (cosign), attach SBOM, push to GHCR | Uses OIDC to GHCR — no stored credentials |
| `.github/workflows/deploy-dev.yaml` | P1 | Deploys to `dev` on merge to `main` | First real CD pipeline |
| `.github/workflows/terraform-plan.yaml` | P1 | `terraform plan` + Infracost comment on PRs touching `infra/` | Cost visibility before merge |
| `.github/workflows/nightly-destroy.yaml` | P1 | Destroys `aws-lab` on a schedule | The cost control that makes the AWS lab affordable |
| `.github/workflows/backlog-sync.yaml` | P0 | Runs `HEL-0.6.1`'s script on backlog changes | Keeps the board in sync automatically, not manually |
| `.github/workflows/preview-env.yaml` | P2 | Creates/destroys a namespace per PR | The preview-environment automation |
| `.github/dependabot.yml` or `renovate.json` | P7 | Automated dependency updates | Reduces toil — an explicit SRE mandate |
| `.github/CODEOWNERS` | P7 | Duplicate of root, GitHub-specific location | GitHub only reads it from specific paths |
| `.github/ISSUE_TEMPLATE/` | P0 | Bug/incident templates | Consistency once the repo is public |

---

## 12. `scripts/` — small operator-owned automation

| Path | Phase | What it is | Why it exists |
|---|---|---|---|
| `scripts/backlog-sync.sh` (or `.py`) | P0 | Reads `backlog/*.yaml`, calls `az boards`, links dependencies | Story `HEL-0.6.1` — replaces the throwaway seeder |
| `scripts/verify-image-standard.sh` | P0 | Checks non-root, no-shell, size ceiling, multi-arch | Story `HEL-0.3.4`; becomes a CI gate in Phase 1 |
| `scripts/nuke.sh` | P1 | Destroys every cloud resource across all providers | Tested monthly; the ultimate cost control |
| `scripts/seed-load.sh` | P0 | Wraps `device-sim` with common load profiles | Convenience wrapper around `task load` |
| `scripts/restore-drill.sh` | P6 | Automates the backup-restore-verify sequence | Makes the DR drill repeatable, not a one-off |

---

## 13. Local-only, never committed (for completeness)

Listed so you know they're *expected* to exist locally and are correctly ignored, not
missing.

| Path | Why it's local-only |
|---|---|
| `.env`, `.env.local` | Per-machine secrets and overrides |
| `infra/terraform/**/.terraform/`, `*.tfstate*` | Provider binaries and state — state lives in a remote backend, never in Git |
| `**/node_modules/`, `**/target/`, `**/vendor/`, `**/.venv/` | Language-specific dependency caches |
| `kubeconfig*`, `*.pem`, `*.key` | Credentials and private keys |
| `apps/gen/` (Go), `apps/*/gen/` (TS) | Generated protobuf code — regenerate with `buf generate`, don't commit |

---

## How this maps onto the roadmap

If you want the "what exists after each phase" view instead of the "what's in each
folder" view:

| After phase | You have working... |
|---|---|
| **0** | `apps/`, `docs/`, `backlog/`, root tooling files, `docker-compose.yml` |
| **1** | `infra/terraform/{modules,live/dev,live/stage,live/prod}`, `deploy/kustomize`, first `.github/workflows/`, `observability/` skeleton |
| **2** | `deploy/argocd/`, `deploy/helm/` (one service), `deploy/rollouts/`, `tests/contract/` |
| **3** | `observability/otel`, `observability/dashboards`, `observability/alerts`, `observability/slo`, first `ops/runbooks/` |
| **4** | `deploy/policies/`, `security/` fully populated |
| **5** | `infra/terraform/live/aws-lab`, `infra/terraform/live/gcp-dr`, `infra/bicep/`, `infra/pulumi/`, `infra/ansible/` |
| **6** | `tests/load/`, `tests/chaos/`, `ops/gamedays/`, `ops/dr/` |
| **7** | `platform/`, `infra/crossplane/` |

Nothing here is exhaustive of every future edit — it's the map of what needs to *exist*,
once, for the system to run end to end. Individual files will change constantly; this
document should not need to.
