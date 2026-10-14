---
name: helios-continuation
description: Resume Project HELIOS from the current checkout, or refresh its cross-session handoff when the user says "agent update".
disable-model-invocation: true
---
# HELIOS Continuation Skill

The repository root `AGENTS.md` defines always-on agent behavior. `docs/CONTINUATION.md` is the current progress snapshot and next-task pointer. Read those first; use this skill for continuation-specific workflow only. The status text later in this file is an old snapshot and is not authoritative.

## Canonical repository

Use the Git root of the current workspace as canonical. Do not assume an absolute path, operating system, or sibling/archive location. Before editing, inspect `git rev-parse --show-toplevel` and `git status --short --branch`. Preserve existing user changes; never reset, checkout, delete, or overwrite user work without permission.

---

## Project purpose

HELIOS is a hands-on portfolio project covering the responsibilities of:

- DevOps engineering
- Site Reliability Engineering
- Platform Engineering
- SecOps/DevSecOps
- Cloud architecture
- Infrastructure as Code
- Configuration management
- CI/CD and release engineering
- Kubernetes operations
- Observability and SLOs
- Incident response and disaster recovery
- Resilience engineering
- FinOps

This is not a tool-collection exercise. Every technology must solve a real workload
problem, and major decisions should be recorded as ADRs.

The first lap is Phases 0–7 and is tracked as 100% of the project. Phase 8+ is a separate
advanced pass and must not change the first-lap denominator.

---

## Application: Trackline

Trackline is a multi-tenant fleet telemetry SaaS platform.

Customers register fleets and vehicles. GPS devices send telemetry. Trackline receives
and processes that data, updates live positions, stores history, detects geofence/speed/
idle/offline events, raises alerts, sends notifications, calculates usage, and later
adds ETA machine learning and batch billing.

Trackline is intentionally complex enough to create real operational pressure:

- synchronous REST APIs
- high-volume gRPC/MQTT ingestion
- Kafka event processing
- real-time position freshness
- asynchronous notifications
- batch billing
- ML inference
- multi-tenant quotas and isolation
- stateful PostgreSQL, Redis, Kafka, and object storage

### Phase 0 services

| Service | Language | Responsibility |
|---|---|---|
| `edge-gateway` | Go | Public entrypoint, JWT validation, tenant context, rate limiting, proxy |
| `ingest` | Go | gRPC telemetry ingestion and Kafka publishing |
| `fleet-api` | TypeScript/NestJS | Tenants, fleets, vehicles, geofences, alerts, PostgreSQL |
| `notifier` | Rust | Email/webhook delivery, retries, HMAC signing, DLQ |
| `device-sim` | Go | Synthetic fleet and load generator |
| `web` | Next.js/React | Dashboard, live positions, alert feed |

Phase 2 adds `stream-proc` with Java/Flink, `eta-ml` with Python/FastAPI, and
`billing-batch` with Java/Spring Batch.

The application source is supplied, but it has not automatically been proven correct.
The operator must compile it, run it, containerize it, test it, load-test it, and debug
it.

---

## Responsibility boundary

The assistant may provide:

- application source code
- schemas, seed data, protobuf, and OpenAPI contracts
- application design and trade-offs
- study topics and self-check questions
- backlog stories, personas, and acceptance criteria
- code review and debugging guidance
- failure-injection ideas
- review of infrastructure written by the operator

The operator builds by hand:

- Dockerfiles and container hardening
- Docker Compose and local runtime setup
- Terraform/OpenTofu/Bicep/Pulumi
- Kubernetes, Helm, Kustomize, and Argo CD
- CI/CD pipelines
- workload identity and secret delivery
- security policies, admission, and network controls
- dashboards, alerts, SLOs, and runbooks
- load tests, chaos, backup/restore, and DR
- Ansible/configuration management
- Backstage, Crossplane, golden paths, and scorecards

Do not write the operator's infrastructure implementation unless the operator explicitly
changes this agreement. Explain, review, critique, and validate their work instead.

---

## Constraints

- macOS Apple Silicon, `arm64`
- 16 GB RAM
- OrbStack is the intended local container runtime
- Multi-architecture images are required: `linux/arm64` and `linux/amd64`
- Local multi-cluster Kubernetes is not practical
- Approximately `$0–15/month` cash budget
- Azure: recurring `$50/month` MSDN credit and always-on production target
- AWS: ephemeral lab only; destroy nightly because EKS control-plane cost is expensive
- GCP: do not create until Phase 4; its `$300` credit has a 90-day clock
- Cloudflare free tier: DNS, WAF, CDN, Tunnel, and edge layer
- GitHub repository: public, with GitHub Actions and GHCR
- Lab domain: `kshitizmichael.xyz`, purchased only at Phase 1
- Personal domain: `kshitizmichael.in`, kept separate from lab experiments

Azure is the always-on production tier because AKS control-plane cost fits the recurring
credit. AWS is the ephemeral depth lab. GCP is deliberately delayed until the project is
ready to use the limited credit.

---

## Repository structure

```text
apps/             application source, contracts, protobuf, schema, seed data
backlog/          YAML backlog source of truth
docs/             charter, architecture, roadmap, study guide, working agreements
infra/            operator-owned IaC and Ansible
deploy/           operator-owned Kubernetes, Helm, Argo, and policies
observability/    OTel, Prometheus, dashboards, alerts, SLOs
security/         scans, SBOM, threat models, policy inputs
platform/         Backstage, golden paths, scorecards, Crossplane
ops/              runbooks, postmortems, game days, DR records
tests/            load, chaos, and contract tests
scripts/          operator-owned automation
.github/          CI/CD workflows and repository configuration
```

Do not reread the entire repository to resume. Start with `docs/CONTINUATION.md`, then inspect only the relevant roadmap/backlog section and files for the active task. The roadmap defines order; local backlog YAML defines acceptance criteria.

---

## Work tracking

Use the checked-in backlog and roadmap as the working plan. The user has asked to defer ADO synchronization; do not access or update ADO unless explicitly asked. Only one phase should be groomed ahead.

---

## Current toolchain

The canonical repository has `.mise.toml` with these pinned versions:

```toml
[tools]
go = "1.25.12"
node = "22.20.0"
python = "3.13.15"
rust = "1.88.0"
kubectl = "1.35.9"
helm = "3.19.4"
terraform = "1.14.9"
k6 = "1.2.3"
gitleaks = "8.30.1"
buf = "1.73.0"
task = "3.53.1"
k3d = "5.9.0"
pnpm = "12.8.2"
```

Do not silently change these versions. If a version must change, explain compatibility
and update `.mise.toml` deliberately.

---

## Current repository security setup

The canonical repository contains:

- `.githooks/pre-commit`
- `security/gitleaks/gitleaks.toml`
- `.mise.toml`

The current hook is:

```sh
#!/bin/sh
set -eu
exec mise exec -- gitleaks git --pre-commit --staged --redact -c security/gitleaks/gitleaks.toml
```

Before marking secret-scanning work complete:

1. Confirm the Git root is this HELIOS checkout.
2. Confirm `git config --get core.hooksPath` returns `.githooks`.
3. Confirm `.githooks/pre-commit` is executable.
4. Confirm the gitleaks config exists.
5. Plant a harmless AWS-shaped test value.
6. Stage it and attempt a commit.
7. Confirm the hook rejects the commit.
8. Remove the test fixture completely.
9. Scan this repository's history, not a parent workspace's history.
10. Record evidence in `docs/CONTINUATION.md` when the user requests an `agent update`.

Do not claim success merely because gitleaks prints `no leaks found`. Confirm that the
pre-commit hook actually ran and rejected a staged test secret.

---

## Historical snapshot (stale; do not use as current status)

This section is retained only as historical context. Do not follow its status or next actions. Use `docs/CONTINUATION.md` and verify relevant facts in the current checkout. Its old canonical
root was `/Users/kshitizmichael/Helios`, branch
`feature/Initial-setup`, remote `https://github.com/kzmichael/Helios.git`. The
worktree still has substantial existing changes, including modified `LICENSE` and
`README.md` and many untracked project files. Preserve everything; do not commit,
push, clean, or rewrite history without an explicit request. The old
`Limitless/Major Project/helios` copy remains inactive.

### Project/tooling and tracking

- All five Phase 0 learning spikes (#28–#32) were reported Closed in ADO; source
  delivery (#23) was also reported Closed. Neither establishes runtime acceptance.
- The latest ADO state independently observed was from 2026-10-08: #24 Closed,
  #25 Active, and #26, #27, #33–#36 New. This is stale and has not been refreshed
  for this snapshot. Do not update or close ADO items without explicit request and
  acceptance evidence.
- `.mise.toml` still pins Rust 1.88.0. Notifier's lockfile now pins `uuid` 1.26.0
  (MSRV 1.85), avoiding the Rust 1.89 requirement of `uuid` 1.28.0. `cargo fetch
  --locked` succeeded. This does not prove `cargo test` or notifier runtime works.
- The gitleaks hook/config are present and the hook path was previously verified,
  but the staged test-secret rejection is still unverified. A canonical-ref scan
  was clean; its staged scan covered zero bytes. Do not stage a real secret.
- OrbStack served Docker and the active buildx builder advertised arm64 and amd64
  in the last check. Test-container/idle resource-baseline evidence remains
  unverified; an obsolete `desktop-linux` builder had reported an error.

### Containerization evidence

Treat user-reported results separately from terminal-verified results:

- **edge-gateway:** its Dockerfile is populated, uses a Go multi-stage build and
  Distroless static nonroot runtime, and exposes 8080. No current build or
  acceptance result was checked in this update. The backlog makes it the
  reference image; verify it before considering the image set complete.
- **ingest:** its Dockerfile is populated and requires the `apps/` build context
  because it uses the sibling `gen/go` module. The operator previously reported
  a successful image build, but current hardening/runtime acceptance was not
  checked here.
- **device-sim:** its Dockerfile is populated. Earlier user-provided evidence
  reported a successful build, UID 65532, ~8.2 MB size, no shell, and a stop log;
  container exit/OOM evidence was not provided, and telemetry was not integrated
  because ingest/Kafka were unavailable.
- **fleet-api:** its Dockerfile builds with pinned pnpm and runs on
  `gcr.io/distroless/nodejs22-debian12:nonroot`. Terminal evidence: image size
  54,913,829 bytes, configured UID 65532, no `/bin/sh`, successful startup, and
  after app-side SIGTERM/pool-shutdown changes, `docker stop -t 10` ended with
  `exit=0 oom=false`. Its Nest build also passed. This verifies startup/shutdown,
  not database readiness or full stack integration. Local dev keys must be
  mounted read-only; never copied into the image.
- **notifier:** Docker build succeeded. Terminal evidence: image size
  11,771,585 bytes, configured UID/GID 65532, and no `/bin/sh`. Its first runtime
  attempt failed with `libz.so.1` missing in the Distroless runtime. The current
  Dockerfile copies only the executable into `distroless/cc`; investigate shared
  library dependencies (start with `ldd` in the build stage) and include only
  required runtime libraries. Do not claim startup, graceful SIGTERM, Kafka
  delivery, or full `HEL-0.3.2` acceptance until re-tested. `notifier/src/main.rs`
  currently has an unbounded receive loop without explicit SIGTERM handling.
- **web:** `apps/web/Dockerfile` is still empty at the latest file check.
- `apps/notifier` now has `Cargo.lock`; the lockfile is required for
  `cargo build --locked`. There is no shared `apps/.dockerignore` from the latest
  check; `fleet-api` has its own service-local `.dockerignore`.

### Next actions

The immediate notifier blocker is runtime linkage: diagnose `libz.so.1`, rebuild,
then rerun its startup and SIGTERM checks. After notifier passes, the web image is
the next unbuilt Dockerfile. Before declaring the six-image story `HEL-0.3.2`
complete, gather each service's build result, nonroot identity, no-shell result,
size, SIGTERM-under-10-seconds result, and build/cache timing. Multi-architecture
validation (#35) and the hardening checker (#36) are subsequent stories; a passing
single-architecture image build is not proof of telemetry delivery.

The operator owns Dockerfiles and platform validation. Guide and review their work;
do not write those artifacts or run acceptance commands for them unless they
explicitly change that boundary.

---

## Working style

- Before editing, state one local hypothesis and one cheap check.
- Make the smallest reversible change that tests the hypothesis.
- Validate immediately after each substantive edit.
- Never reset, checkout, delete, or overwrite user changes without permission.
- Do not commit, create branches, or push unless explicitly requested.
- Do not provision cloud resources without discussing cost, account, and cleanup impact.
- Prefer repository conventions over new abstractions.
- Use ASCII by default.
- Keep comments only for non-obvious behavior.
- Do not reveal the deliberately seeded application weaknesses before the operator discovers
  them through testing and observability.
- When blocked, inspect the concrete error and repair the same slice before expanding scope.

## `agent update` workflow

When the user says `agent update`, follow the root `AGENTS.md` procedure and refresh `docs/CONTINUATION.md` from the current conversation, Git status/diff, and actual validation results. Separate user-reported completion from independently verified results, identify blockers, and leave one immediate next task with its acceptance checks. Do not commit or push; the user will push from the machine they are using. Do not consult ADO unless asked.

At session start, read the handoff, verify the current Git root/status, and continue from its next task. If the user gives a different task, follow that instruction and update the handoff when asked.
