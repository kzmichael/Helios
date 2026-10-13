# Roadmap and Execution Guide

This document is both the **sequence of work** and the **completion tracker** for the
first complete HELIOS lap. It is deliberately explicit about what to do, why that order
exists, and what evidence proves completion. It does not provide the infrastructure code
or do the operator's work.

Use this with:

- `START_HERE.md` for the plain-language explanation of the whole project.
- `05_STUDY_GUIDE.md` for the topics to study before a step.
- `06_WAYS_OF_WORKING.md` for story rules and the ADO board.
- `07_REPOSITORY_BLUEPRINT.md` for exact paths and file purposes.
- `backlog/phase-0.yaml` and the [Helios ADO board](https://dev.azure.com/Kugleblitz712/Helios)
  for work-item status and dependency links.

---

## How to use this guide

1. Work phases in order. Do not provision later-phase infrastructure just because it is
  interesting; later phases depend on evidence and foundations from earlier ones.
2. Within a phase, follow the numbered steps unless a step is explicitly marked
  **parallel**. Dependencies are real technical, learning, budget, or safety constraints.
3. For each step, study the linked topics, implement the work yourself, run the listed
  checks, and save the evidence. A green command without understanding the result is not
  a completed learning step.
4. Move the corresponding ADO work item only after its acceptance criteria pass. The
  board is the execution record; this roadmap defines the sequence and progress weights.
5. At phase end, complete the phase gate and closing artifacts before starting the next
  phase.

### Responsibility boundary

The operator builds the repository tooling, containers, Compose stack, cloud resources,
IaC, Kubernetes manifests, pipelines, security controls, observability, tests, policies,
runbooks, and platform. The application source and its API/data contracts are supplied in
`apps/`. Reading, building, testing, debugging, and extending that source are part of the
operator's work. The application source being delivered does **not** mean it has been
compiled or validated in the operator's environment.

### Work-in-progress rule

Keep at most **two active work items**, ideally one study spike and one small setup story.
Do not start a dependent item before its predecessors are done. When stuck, record the
blocker in ADO and continue only with an independent item; do not silently change the
dependency order.

---

## Project progress tracker

### What "100%" means

The first lap is **Phases 0–7** and is exactly 100% of the first-lap project. Phase 8+ is
an optional advanced pass with its own cycle tracker; it does not keep moving the first-
lap denominator. This gives the project a real finish line while leaving room to continue
learning afterward.

The phase weights below are planning weights, not claims of scientific precision. They
roughly reflect relative implementation, verification, and operational effort. They are
fixed for this lap so the percentage does not change whenever the backlog is groomed.

| Phase | Weight of first lap | Cumulative if phase and every earlier phase pass |
|---|---:|---:|
| 0 — Foundations and local application | 14% | 14% |
| 1 — First production | 18% | 32% |
| 2 — GitOps and progressive delivery | 12% | 44% |
| 3 — Observability, SLOs, on-call | 14% | 58% |
| 4 — Security and supply chain | 12% | 70% |
| 5 — Multi-cloud and configuration management | 12% | 82% |
| 6 — Resilience, scale, FinOps | 10% | 92% |
| 7 — Platform engineering | 8% | **100%** |

### Calculation

Each phase has numbered steps with weights that sum to 100 within that phase. A step's
weight is earned only when its completion evidence is checked and the linked work item is
accepted. `Active`, `Blocked`, and partially complete steps earn **zero** until their
acceptance evidence is complete. This avoids a subjective "almost done" percentage.

Formula:

`first-lap progress = sum(phase weight × completed step weight ÷ 100)`

Example: if Phase 0 is complete, and Phase 1 steps worth 25% of that phase are accepted,
progress is `14% + (18% × 25%) = 18.5%`.

Do not count an entire phase as done until its phase gate passes. Do count accepted steps
inside an in-progress phase. Keep the ADO item as the detailed state, and update the
phase-step checkboxes here when acceptance evidence exists.

### Current baseline — 2026-10-01

This is a snapshot, not a permanent status claim. Refresh it from the ADO board when work
changes.

| Work | Current evidence | Tracker state |
|---|---|---|
| Application source (`HEL-0.0.1`, #23) | Source delivered; not yet built or run locally | Delivered input; execution/verification remains in Phase 0 steps |
| Learning spikes (`SPK-0.1` through `SPK-0.5`, #28–#32) | User reports all five studies completed; ADO items marked Closed | Complete by user report; attach self-check notes/evidence when available |
| Public repository (`HEL-0.1.1`, #24) | ADO item is Closed based on the reported repo creation; current workspace remote still points to ADO `Ferrai/Limitless`, and project-level ignore/toolchain files are not verified | Treat the board item as reported complete; do not push the whole parent workspace until the repo boundary is verified |
| Toolchain and runtime setup (`HEL-0.1.2`–`HEL-0.1.4`, #25–#27) | `.mise.toml` and `Major Project/.gitignore` are missing locally; OrbStack baseline is not verified | **Next implementation work** |

**Starting point:** Phase 0 implementation. All learning spikes are reported complete.
The next work is to make the project repository safe and reproducible: verify the public
repo boundary, pin the toolchain, add secret scanning, establish the container runtime,
then build the first reference image. See the next-action box at the end of this guide.

---

## Standard completion evidence

Save evidence where another person can inspect it. Depending on the step, evidence can
include:

- The exact command run and its exit status, recorded in the ADO item or a short phase log.
- A test result or generated report committed to the appropriate `tests/` or `ops/` path.
- A screenshot only when the outcome is inherently visual (for example, a dashboard or
  browser certificate view); pair it with a reproducible command where possible.
- An ADR for a meaningful design choice, including at least one rejected alternative and
  one accepted downside.
- A runbook for any operational procedure that you may need to repeat.

Never commit private keys, tokens, cloud state, kubeconfigs, `.env` files, or real
customer data as "evidence". Evidence must not create a new security incident.

---

# First lap: Phases 0–7

## Phase 0 — Foundations and the local application

**Phase weight:** 14%  
**Goal:** Run the Trackline core locally, over HTTPS, from a clean checkout, with a load
generator and a reproducible operator interface. No cloud resources and no purchased
domain are required.

**Inputs supplied:** six core services (`edge-gateway`, `ingest`, `fleet-api`, `notifier`,
`web`, `device-sim`), protobuf/OpenAPI contracts, database schema and seed data, and
service READMEs.  
**Operator builds:** repo foundation, pinned tools, hardened images, Compose stack, local
TLS, developer workflow, validation, and phase evidence.

**Study before/during the phase:** Linux processes/cgroups/namespaces, defensive Bash,
public-repo safety, PKI basics, container layers/builds/hardening, Docker Compose basics.
Use the Phase 0 section of `05_STUDY_GUIDE.md` and the Phase 0 spikes in ADO.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 0.1 Safe starting point | 8% | Complete repository-safety spike `SPK-0.5`. Confirm which exact directory will become the public project repo. Create/verify the public repo, enable MFA and PR protection, add `.gitignore` and `.gitattributes`, then make a fresh clone and verify the files. | Public exposure is hard to reverse. Ignore rules and repo boundaries must exist before application files or generated credentials are pushed. | Fresh clone is clean; a test `*.tfstate` and `.env` are ignored; repository URL and protection settings are recorded. Never publish the whole parent `Limitless` workspace. |
| 0.2 Learn the runtime foundations | 10% | Complete the Linux spike and answer its self-check; complete Bash and PKI spikes. Run the exercises in the study guide, not just reading. | Containers and TLS are built on these concepts; studying them first reduces blind debugging later. | ADO spikes accepted; self-checks answered from memory; notes/evidence are in the existing learning notes. |
| 0.3 Pin tools and scan for secrets | 8% | Add the exact tool versions in `.mise.toml`; run `mise install` and `mise doctor`; install/configure the gitleaks pre-commit hook; scan repository history. | Reproducible tools and early secret detection need to precede builds and public commits. | Clean tool install passes; planted test secret is detected and removed before commit; history scan is clean. The hook is convenience, not the later CI security control. |
| 0.4 Measure the local runtime | 8% | Install/configure OrbStack; run a test container; record idle memory and architecture support. | This Mac has 16 GB RAM. Measure available capacity before selecting how much of the stack runs together. | `docker run --rm hello-world` passes; buildx reports `linux/arm64` and `linux/amd64`; baseline is recorded. |
| 0.5 Learn and build container images | 14% | Complete `SPK-0.2`. Build the reference Go image for `edge-gateway`; inspect its layers and runtime user; then apply the standard to the other five services; add both architectures; write the image-verification script. | One reference image limits repeated mistakes. The verification script turns a written standard into a check. | Every service builds; user is non-root; no shell is present; graceful SIGTERM works; image sizes are recorded and justified; both architectures are present; verifier catches a deliberate violation. |
| 0.6 Bring up stateful dependencies | 10% | Write Compose definitions for PostgreSQL, Redis, Kafka in KRaft mode, and Mailpit/webhook sink. Add health-based readiness checks and named volumes. | The data layer must be healthy before application services start; fixed sleeps hide timing defects. | Cold start reaches healthy state; Kafka has no ZooKeeper; data survives restart and disappears after an intentional volume reset. Record RAM use. |
| 0.7 Apply schema and seed data | 8% | Select and wire the local migration runner; apply numbered migrations to an empty database; apply deterministic seed data; query counts and relationships. | Running the SQL is the first real validation of the supplied data model. Migrations now avoid a costly retrofit in Phase 2. | A clean database builds from zero; migrations fail closed on an invalid migration; seed counts and all three tiers are verified; a second clean rebuild reproduces the same logical data. Record defects and fix them with new migrations rather than silently altering applied history. |
| 0.8 Wire the application and load | 12% | Start the services using health-based dependencies; configure only local test credentials; connect browser/API to the stack; start 500 devices with a mixed profile; route notifications only to local sinks. | This is the first end-to-end product. It creates a baseline before cloud, GitOps, or observability complexity is added. | A fleet/vehicle view loads; telemetry reaches the documented storage/queue path; notifications are visible in Mailpit/local webhook sink; 30-minute run has no crash or unbounded growth; memory is recorded. Clearly label any source endpoint that is not implemented rather than simulating success. |
| 0.9 Add local TLS | 8% | Complete the PKI self-check; create/install a local CA with `mkcert`; assign local names; terminate TLS at the local reverse proxy; verify HTTP redirect, cookies, and WebSocket upgrade where implemented. | Production uses HTTPS. Local HTTPS exposes certificate and browser-security failures before deployment. | Browser trusts the certificate; SANs cover local names; HTTP redirects; secure cookies behave correctly; CA private key is ignored by Git. No public domain needed. |
| 0.10 One-command workflow and clean rebuild | 8% | Write the single `Taskfile.yml` interface (`up`, `down`, `build`, `test`, `lint`, `logs`, `seed`, `load`, `nuke`); add a devcontainer; document prerequisites; remove local state/images/caches and follow only the docs to rebuild. | A fast inner loop matters, but reproducibility must be demonstrated from a clean state. | One command starts the stack; one command destroys local volumes/network; devcontainer works; clean rebuild from docs succeeds; cold-start duration and any fixes are recorded. |
| 0.11 Failure exercise and phase close | 6% | Deliberately fill a disposable filesystem and exhaust a controlled DB pool; diagnose without restarting; write ADR, phase retro, architecture/request-flow diagrams, update runbooks; run the exit review. | A configured control is not proven until exercised; phase closure converts experience into reusable knowledge. | Failure cause and recovery are recorded; exit gate below passes; ADR, retro, diagram, and runbook changes are committed. |

### Phase 0 phase gate

- [ ] Clean checkout can start the documented local stack using the Taskfile.
- [ ] 500 simulated devices produce visible telemetry for at least 30 minutes.
- [ ] All images are non-root, have no shell, are multi-arch, and meet the agreed size
    limits or have an accepted ADR explaining exceptions.
- [ ] Local browser traffic uses a trusted local CA certificate; private key is not in Git.
- [ ] Schema and seed have been executed and their results inspected.
- [ ] Container/disk or pool failure is diagnosed from evidence without restart-first
    guesswork.
- [ ] Phase 0 self-checks pass; ADR, retro, and runbook/diagram updates are committed.
- [ ] Phase 1 backlog is groomed and budget/account checks are current.

**Phase completion:** earn the remaining Phase 0 weight only after this gate passes.
 
---

## Phase 1 — First production

**Phase weight:** 18%  
**Goal:** A real Trackline endpoint on a real domain with public TLS, deployed to Azure
from GitHub Actions without static cloud credentials. The environment must fit the
monthly credit and be destroyable/rebuildable.

**Study first:** CIDR/subnetting, DNS resolution, TLS/ACME, Kubernetes core objects and
probes, Terraform state/modules, GitHub Actions permissions/OIDC, Azure identity and cost.
Complete the relevant study-guide checks before provisioning.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 1.1 Budget and safety controls | 12% | Confirm MSDN credit/region/quota; make a cost estimate; set Azure budget alerts; create a documented kill-switch plan; ensure remote state is private, encrypted, and locked; add plan review and cost estimate checks before apply. | Guardrails precede resources. The credit is a hard cap, not a target to spend. | Budget alarms verified; backend locking tested; no secrets/state in public Git; a reviewer can identify the command that destroys lab resources. |
| 1.2 Domain and identity preparation | 8% | Buy `kshitizmichael.xyz` only now; delegate DNS to Cloudflare; enable registrar protections; create the planned DNS records; document DNS resolution and TTLs. Provision/select the Phase 1 OIDC provider from the existing auth design. | Phase 0 needs neither domain nor cloud account; Phase 1 needs public DNS and ACME. | Nameservers resolve through Cloudflare; DNS records are documented; issuer/JWKS are reachable; never commit registrar/cloud credentials. |
| 1.3 Network and state foundation | 15% | Choose non-overlapping address ranges for future Azure/AWS/GCP/on-prem; create Azure resource group, VNet/subnets/NSGs, remote state, and required private/public routing in Terraform. | IP overlap is painful to repair and Phase 5 hybrid networking depends on this choice. | `terraform plan` is reviewed; apply succeeds; state lock tested by concurrent plan/apply attempt; network diagram updated; cost review passes. |
| 1.4 AKS and minimal compute | 15% | Create AKS with the cost-approved node pool (spot where acceptable); configure identities/RBAC, node labels/taints if needed, cluster access, and basic system components. Avoid managed services not budgeted in `04_ENVIRONMENTS.md`. | Establish a controlled cluster before deploying workloads; don't add managed Redis/Kafka simply because they are convenient. | Cluster is created only by Terraform, operator can access using least privilege, node spend is understood, and destroy/recreate succeeds in a test environment. |
| 1.5 Deploy the minimum working application | 15% | Build/apply base Kubernetes manifests: namespace, config, secrets mechanism, Deployments, Services, resource requests/limits, startup/readiness/liveness probes, PDB where meaningful; deploy dependencies using the agreed cost topology. | Deploy the smallest valuable slice first; expand only after health is observable. | The API and UI are reachable inside the cluster; probes reflect real readiness; resource limits exist; test request and telemetry flow pass. |
| 1.6 Public entry, DNS, and certificates | 10% | Configure Cloudflare DNS/WAF; choose the shared ingress/Gateway endpoint consistent with budget; install cert-manager; create DNS-01 credentials using a secret-delivery method; issue and renew a certificate. | Public TLS builds on the Phase 0 trust-chain exercise and is required for the real endpoint. | External HTTPS works; certificate chain/SAN/expiry inspected; renewal test succeeds; no per-service load balancers are created accidentally. |
| 1.7 CI build, scan, and OIDC deploy | 15% | Create GitHub Actions with least-privilege permissions; build/test/image-scan; push immutable image tags to GHCR; federate via OIDC; deploy using the approved Phase 1 method; protect production environment. | The pipeline comes after a reproducible manual IaC/deployment path exists; automation should encode a path already understood. | A commit/PR produces tested image; OIDC token exchange succeeds; no static cloud keys; merge deploys; failed checks block deploy; commit-to-prod timeline captured. |
| 1.8 Monitoring and budget drill | 5% | Install Prometheus/Grafana; add one RED dashboard and actionable alert; test the budget alert and application alert routes. | Minimum observability must exist before deliberate failure or calling this production. | Dashboard and alert are provisioned from Git; alert fires and reaches you; runbook link works; cost notification is confirmed. |
| 1.9 Certificate-failure drill and close | 5% | In a disposable/non-user-facing endpoint, simulate expiry/renewal failure safely; recover; record evidence; complete ADR/retro/runbook. | Exercise certificate operations without intentionally taking the only production endpoint down. | Renewal failure is detected; recovery time and steps recorded; phase gate passes. |

### Phase 1 phase gate

- [ ] Public HTTPS endpoint serves the app with a valid, automatically renewed certificate.
- [ ] CI deploys through OIDC; no long-lived cloud credentials exist in GitHub or the
    cluster.
- [ ] Terraform state is remote/private/locked; infra can be rebuilt from code.
- [ ] Budget alarms and a tested destroy/kill-switch procedure exist before scaling up.
- [ ] At least one actionable dashboard/alert has been exercised with a runbook.
- [ ] Real monthly cost has been checked against the $50 credit; phase artifacts exist.

---

## Phase 2 — GitOps, environments, and progressive delivery

**Phase weight:** 12%  
**Goal:** Git becomes the deployment source of truth; releases are tested, promoted,
observable, and automatically reversible.

**Study first:** pull vs push CD, Argo CD reconciliation, Kustomize overlays, Helm release
semantics, canary analysis, database expand/contract migration, SOPS/External Secrets.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 2.1 Define deployment repository boundaries | 10% | Decide where app source, environment config, and secrets references live; document the repo topology in an ADR; ensure CI no longer needs cluster-admin credentials. | GitOps repository shape affects permissions and ownership; settle it before wiring reconciliation. | ADR compares viable layouts; access boundary documented; no cluster credential in CI. |
| 2.2 Install Argo CD and bootstrap reconciliation | 15% | Install Argo CD; create root app-of-apps; configure project RBAC, sync policy, health checks, and safe initial bootstrap. | Establish one reconciler before migrating services to it. | Fresh cluster can install Argo from documented bootstrap; desired state matches Git; secrets are not plaintext. |
| 2.3 Create Kustomize base and environment overlays | 15% | Move shared manifests to base; add dev/stage/prod overlays; keep only intentional differences in patches; validate rendered output. | Reduces environment drift before promotion workflows exist. | `kustomize build` succeeds for each overlay; diff explains every env-specific difference. |
| 2.4 Add Helm comparison and secret handling | 10% | Package one representative service with Helm; compare templating and Kustomize; choose SOPS/age or External Secrets and document bootstrap/rotation. | Compare tools on the same workload, not as disconnected tutorials. | Both render valid resources; ADR records trade-offs; secret rotation/bootstrap demonstrated without committing secrets. |
| 2.5 Add promotion and schema migration workflow | 15% | Promote immutable image digests through dev→stage→prod; add Atlas/Flyway migrations using expand/contract; gate production on checks and approvals. | Safe promotion and backwards-compatible schema change are prerequisites to canary. | Upgrade and rollback across app versions work without downtime; migration tested from clean and prior schema. |
| 2.6 Preview environments | 15% | Create one isolated namespace per PR; inject only test credentials/data; provision route/certificate; attach TTL/cleanup; delete on PR close. | Ephemeral environments are useful only after base manifests and automated reconciliation work. | PR opens preview; test passes; PR close removes namespace, DNS/route, and resources; cleanup failure alerts. |
| 2.7 Canary and automated rollback | 15% | Add Argo Rollouts; select metric query and analysis window; start with small traffic; define abort and rollback behavior; run bad-canary drill. | Rollback must be driven by user-impact metrics, not pod health alone. | Bad canary is automatically rolled back; alert/timeline captured; good canary promotes; no manual intervention. |
| 2.8 Second CI system and close | 5% | Mirror one service build/test pipeline in Azure DevOps or GitLab CI; compare shared concepts; write ADR/retro/runbooks. | A bounded comparison teaches portability without duplicating every pipeline. | Same artifact/test result; differences documented; phase gate passes. |

### Phase 2 phase gate

- [ ] Git is authoritative; cluster drift is detected and corrected.
- [ ] A PR obtains a working preview and teardown is verified.
- [ ] A bad canary rolls back automatically based on an agreed metric.
- [ ] Database schema change works across old/new app versions with no downtime.
- [ ] Secrets bootstrap and rotation are documented and exercised.
- [ ] One equivalent pipeline exists in a second CI system; phase artifacts exist.

---

## Phase 3 — Observability, SLOs, and on-call

**Phase weight:** 14%  
**Goal:** Operate the product through user-centered indicators, traces, useful alerts,
runbooks, and a real response loop.

**Study first:** signals and cardinality, OTel SDK/Collector/context propagation, RED/USE,
PromQL, SLI/SLO/error budgets, burn-rate alerts, incident roles and postmortems.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 3.1 Define the signals and cost limits | 10% | Decide sampling, retention, cardinality budgets, PII exclusions, and which signals each service emits. | Observability can create a large bill and leak data; set bounds before instrumentation floods backends. | Signal and retention ADR; high-cardinality/PII checklist reviewed. |
| 3.2 Instrument services with OpenTelemetry | 20% | Add OTel SDKs for Go, TypeScript, Rust, and later service languages; propagate W3C trace context over HTTP, gRPC, and Kafka; emit structured logs with trace IDs. | Instrumentation before dashboards gives dashboards consistent signals. | A trace crosses gateway/API and asynchronous Kafka boundary; logs correlate by trace ID. |
| 3.3 Deploy the collector and local backends | 15% | Configure OTel Collector receivers/processors/exporters; deploy Prometheus, Grafana, Loki, Tempo; create versioned dashboards. | Collector separates app instrumentation from vendor endpoints and enables routing/sampling. | Collector health visible; metrics/logs/traces arrive; configs and dashboards are in Git. |
| 3.4 Prove vendor portability | 10% | Send the same OTel stream to Grafana Cloud and Azure Monitor; set per-exporter controls and verify no duplicate/unbounded export. | A second backend demonstrates that app instrumentation is not vendor-specific. | Same trace/metric can be found in both backends; cost/quota usage recorded. |
| 3.5 Define and publish five SLOs | 15% | Choose user-journey SLIs; specify good/valid events, windows, exclusions, and targets; define five SLOs as code, including position freshness. | SLOs must measure what the customer experiences, not host utilization. | Definitions are reviewed, tested against sample data, and rendered on a dashboard. |
| 3.6 Error budgets and actionable alerting | 15% | Write the budget policy; add multi-window/multi-burn-rate alert rules; distinguish page/ticket/dashboard; route and silence deliberately. | Alerts need explicit action and severity; budget policy defines what happens when reliability degrades. | Burn-rate test fires; each page has owner/action/runbook; noisy alert is fixed, not merely silenced. |
| 3.7 On-call and seeded cardinality failure | 10% | Configure an on-call rotation; carry the pager; safely reproduce the seeded high-cardinality problem in a bounded test; stop ingestion before resource exhaustion; fix and add a guard. | SRE behavior is learned by responding, but experiments must have blast-radius controls. | Incident timeline, before/after cardinality, alert, recovery, and postmortem recorded. |
| 3.8 Runbooks and close | 5% | Ensure each page links to an actionable runbook; write postmortems and ADR/retro; review SLO quality. | The next responder should not need oral history. | Stranger-readable runbooks; phase gate passes. |

### Phase 3 phase gate

- [ ] One trace covers device→ingest→Kafka→API→map, including async context.
- [ ] Five SLOs and multi-window burn alerts are live and have fired in a controlled or
    genuine event.
- [ ] Every page has a runbook and a clear action; non-actionable alerts are removed.
- [ ] OTel data reaches at least two backends with cost and retention controlled.
- [ ] Cardinality failure was safely reproduced, corrected, and documented.
- [ ] You have carried the pager and recorded incident learning; phase artifacts exist.

---

## Phase 4 — Security and supply chain

**Phase weight:** 12%  
**Goal:** The platform can prove what it runs, reject artifacts that violate policy, and
authenticate workloads without permanent credentials.

**Study first:** OAuth/OIDC/JWT validation, workload identity, secret lifecycle, SBOM and
signing, admission control, NetworkPolicy/CNI, STRIDE threat modelling, audit logging.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 4.1 Threat model and identity inventory | 10% | Draw trust boundaries/data flows; list human, device, CI, service, and cloud identities; map permissions and secrets. | You need a threat and identity model before selecting security controls. | STRIDE model, identity inventory, risks ordered by likelihood/impact. |
| 4.2 Remove static CI/cloud credentials | 15% | Federate CI to Azure/AWS/GCP with OIDC; use workload identities for pods; remove old keys; verify audit logs show short-lived principals. | Signing and deploy gates rely on trustworthy identity; do this before claiming no static credentials. | Secret scan and provider audit show no long-lived CI keys; rotation/revocation evidence exists. |
| 4.3 Scan dependencies, images, IaC, and secrets | 10% | Add Trivy, Checkov, gitleaks and dependency scanning to required CI checks; define exception owner/expiry and remediation priority. | Finds known issues before admission policy; exceptions need accountable lifecycle. | Planted test secret and vulnerable test artifact are caught; valid exception expires as configured. |
| 4.4 Produce and sign artifacts | 15% | Generate SBOMs; sign images with keyless Cosign; attach provenance/attestations; verify signatures from a clean environment. | Admission policy needs a verifiable artifact identity to enforce. | Image digest maps to build, signature verifies, SBOM is retrievable, provenance policy passes. |
| 4.5 Enforce admission policies | 15% | Install Kyverno or Gatekeeper; start in audit mode; test policy against compliant/non-compliant manifests; then enforce no unsigned image, no `:latest`, non-root, required labels and limits. | Stage policies carefully so the policy itself does not unexpectedly lock out production. | Negative fixtures are rejected with useful reasons; exceptions are documented and expire. |
| 4.6 Default-deny network and audit pipeline | 15% | Apply default-deny in a test namespace; add only required service-to-service allows; verify DNS/telemetry/health still work; ship cluster/cloud audit logs to Wazuh or Elastic. | Network rules require an observed traffic map; audit pipeline is needed to prove policy decisions. | Denied and allowed paths tested; traffic map committed; policy denials/audit events searchable. |
| 4.7 Vault and short-lived secrets | 15% | Deploy Vault; configure identity-based auth; issue dynamic Postgres credentials to `fleet-api`; test renewal, expiration, revocation, and outage behavior; remove static DB password. | Dynamic credentials are meaningful only after workload identity exists. | App receives short-lived credential, renewal succeeds, revoked credential fails, no static password remains. |
| 4.8 CSPM, GCP clock, and close | 5% | Run Prowler baseline on Azure/AWS; triage findings; only now create GCP account and record credit expiry; complete ADR/retro. | Starts the 90-day GCP credit when Phase 5 is near; avoids wasting the clock. | Findings have owners/status; GCP budget alert exists; phase gate and artifacts pass. |

### Phase 4 phase gate

- [ ] Unsigned and policy-violating images are demonstrably rejected at admission.
- [ ] Image signatures, SBOM, and provenance verify against the built digest.
- [ ] No static cloud/CI key or static database password remains.
- [ ] Default-deny policy is active with documented, tested allowed paths.
- [ ] Audit events are searchable in the SIEM; planted secret is blocked by CI.
- [ ] Threat model and CSPM findings are tracked; phase artifacts exist.

---

## Phase 5 — Multi-cloud, multi-IaC, and configuration management

**Phase weight:** 12%  
**Goal:** Operate a justified hybrid topology across Azure, AWS, GCP, and local/on-prem
without allowing any one provider's credentials or network assumptions to leak everywhere.

**Study first:** AWS IAM/VPC/EKS/S3/RDS/IRSA, GCP Workload Identity Federation/GKE/Cloud
Run, Ansible idempotency and Molecule, Bicep/Pulumi, VPN/tunnel, DNS failover, egress cost.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 5.1 Revalidate budgets and placement | 10% | Assign each workload a provider and reason (latency, managed capability, DR, learning); check GCP credit days remaining; establish per-provider budgets/kill switches and non-overlapping CIDRs. | Multi-cloud without placement rationale is duplication and extra cost. | Placement ADR and updated network diagram; budget notifications tested. |
| 5.2 Build ephemeral AWS lab with Terraform | 20% | Create VPC/routes/security, EKS, RDS/S3 only if justified, IRSA, remote state, tags and TTL; deploy one workload; test destroy and nightly destroy. | AWS is deep practice, but its control-plane/network costs make it ephemeral. | Full apply from empty state and destroy succeed; nightly cleanup runs; cost report and IAM audit saved. |
| 5.3 Build the on-prem/local tier with Ansible | 15% | Prepare target VM(s); write inventory/roles/cloud-init as selected; provision k3s and Cloudflare Tunnel; run Molecule and re-run playbooks. | Configuration management needs machines to configure; local/homelab is the low-cost target. | From clean VM to cluster without manual configuration; second Ansible run reports no changes; Molecule passes. |
| 5.4 Expose and secure the hybrid endpoint | 10% | Configure Cloudflare Tunnel, DNS, TLS, access restrictions, and health checks; verify no inbound ports are exposed unnecessarily. | Connects the local tier to the public topology while preserving the outbound-only tunnel boundary. | Endpoint works over TLS; tunnel loss is visible; access is restricted; no direct origin bypass. |
| 5.5 Build GCP DR/analytics slice | 15% | Federate GitHub Actions to GCP; provision GKE Autopilot as the chosen DR target, Cloud Run for one workload, BigQuery analytics; set budgets and teardown strategy. | Uses the time-limited credit for a deliberate slice, not a duplicate of every Azure service. | OIDC federation works; one workload runs; analytics query succeeds; projected and actual spend reviewed. |
| 5.6 Compare IaC tools on the same boundary | 10% | Reimplement the chosen Azure prod tier in Bicep; use Pulumi for Cloudflare layer; compare state, testing, review, drift, and team ergonomics; avoid having two tools own the same live resource. | Comparison is meaningful only when resource ownership is clear and implementations are comparable. | Bicep and Pulumi plan/deploy/destroy their own test scopes; ADR states which tool owns each layer. |
| 5.7 Configure health-based failover | 15% | Add Cloudflare health checks/load balancing; define primary/secondary semantics, data consistency expectations, rollback/failback steps, RTO and RPO; conduct controlled Azure outage drill. | Traffic switching is only useful when data and recovery semantics are understood. | Controlled failover and failback are timed; RTO/RPO measured; runbook and logs retained. |
| 5.8 Cost review and close | 5% | Review cross-cloud egress/NAT/idle resource spend; verify destroy schedules; write ADR/retro and update diagrams/runbooks. | Cross-cloud data transfer can erase free-credit assumptions. | All resources have owner/TTL; costs fit cap; phase gate passes. |

### Phase 5 phase gate

- [ ] AWS lab fully rebuilds from code and is destroyed on schedule.
- [ ] Ansible builds the on-prem cluster idempotently; tunnel is secured and documented.
- [ ] A real workload exists on each chosen platform for a documented reason.
- [ ] GCP identity is federated and credit spend is monitored.
- [ ] Azure outage/failover/failback was executed and RTO/RPO measured.
- [ ] Terraform, Bicep, and Pulumi boundaries are explicit and do not co-own resources.
- [ ] Phase artifacts and cost review are complete.

---

## Phase 6 — Resilience, scale, and FinOps

**Phase weight:** 10%  
**Goal:** Quantify capacity and cost; demonstrate recovery and controlled failure; fix the
application's seeded weaknesses using measured evidence.

**Study first:** latency percentiles, Little's Law, load profiles, retries/jitter, circuit
breakers, RTO/RPO, backup/restore, HPA/KEDA/Karpenter or Cluster Autoscaler, unit economics.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 6.1 Establish a baseline and capacity hypothesis | 10% | Run smoke and controlled load tests; record throughput, p50/p95/p99, errors, saturation, node/pod cost; state expected capacity and abort thresholds before ramping. | A test without a baseline or abort condition risks cost and yields weak conclusions. | Versioned baseline report; explicit hypothesis, limits, and stop conditions. |
| 6.2 Build k6 load profiles | 15% | Add smoke/load/soak/spike/breakpoint scripts; use realistic mixed device profiles; assert user-facing thresholds; keep mega-load inside local/VPC boundaries to avoid egress bills. | Different test shapes expose different failure classes; thresholds make regressions repeatable. | Each profile runs; reports include percentiles/errors; CI uses a bounded smoke gate; costs/limits recorded. |
| 6.3 Find and fix seeded application weaknesses | 15% | Scale gradually; identify each seeded problem through signals; capture reproduction before changing code; fix with an application change or platform control as appropriate; re-run the same test. | Learning comes from discovery and evidence, not being handed the answer. | Each weakness has reproduction, root cause, fix, regression test, and postmortem. |
| 6.4 Add lag-based scaling and interruption handling | 15% | Configure KEDA on Kafka lag; configure node autoscaling; test scale-out/in stabilization; interrupt a spot node; verify graceful drain and recovery. | Queue lag is the work backlog signal; CPU can be low while consumers are blocked. | Lag falls after scaling; scale-in does not flap; spot interruption preserves service within the stated objective. |
| 6.5 Write and test chaos hypotheses | 10% | Select Litmus or Chaos Mesh; define steady state, blast radius, abort/rollback, and expected result before each experiment; start with pod kill and latency injection. | Chaos without a prior hypothesis is unstructured damage, not resilience engineering. | At least five bounded experiments have hypothesis, execution evidence, result, and follow-up. |
| 6.6 Verify database and cluster restore | 15% | Create backups; restore Postgres into a clean environment; verify data and application behavior; back up cluster objects with Velero; restore to a newly created cluster. | Backup success is not restore success; fresh-target restoration proves portability. | Timed restore evidence, measured RPO/RTO, data validation queries, and documented gaps. |
| 6.7 Measure per-tenant cost and unit economics | 10% | Deploy OpenCost; allocate namespace/resource cost; combine usage counts with spend; calculate cost per tenant/device/request; identify one measurable saving. | Spend becomes useful when connected to the unit of customer value. | Dashboard shows cost per tenant; allocation coverage and unallocated spend are visible; saving is measured. |
| 6.8 Region-loss game day and close | 10% | Schedule a controlled peak-load region-loss game day; recover/fail over; fail back safely; write incident timeline/postmortem/ADR and update capacity model. | Combines the phase's controls under realistic operational pressure. | RTO/RPO met or remediation stories exist; capacity and monthly cost assumptions updated; gate passes. |

### Phase 6 phase gate

- [ ] Capacity statement includes devices, node count, throughput, latency, and monthly cost.
- [ ] Seeded weaknesses have reproducible tests and fixes.
- [ ] Database and cluster restore succeeded into fresh targets and were timed.
- [ ] Five chaos experiments and one region-loss game day have evidence and postmortems.
- [ ] Lag-based scaling and spot interruption behavior are demonstrated.
- [ ] Cost per tenant is visible and at least one cost decision is evidence-based.

---

## Phase 7 — Platform engineering

**Phase weight:** 8%  
**Goal:** Package the operational platform as a usable product for another engineer, with
catalog, golden path, self-service infrastructure, and measurable quality.

**Study first:** Backstage catalog/scaffolder, platform-as-product, Crossplane providers/
compositions/claims, scorecards, developer-experience feedback, DORA metrics and caveats.

### Ordered steps

| Step | Weight | What to do, in order | Why this is here | Done when / evidence |
|---|---:|---|---|---|
| 7.1 Define platform users and service catalog | 15% | Identify developer personas; add every service, owner, lifecycle, dependencies, docs and operational links to Backstage; import TechDocs. | A platform starts with users and inventory, not with a portal installation. | Catalog has no unowned service; dependency graph matches runtime; docs are browsable. |
| 7.2 Define the golden path contract | 15% | Specify inputs/outputs for a new service; template repo, language/runtime choice, ownership, required checks, security/observability defaults, and escape hatch. | Prevents the template becoming a mandatory golden cage. | New-service design is reviewed by a developer persona; defaults and extension points are documented. |
| 7.3 Build and test the scaffolder | 25% | Generate a service repo with CI, image build/signing, Argo app, policies, dashboard, SLO/runbook skeleton, catalog entry, and preview environment. | The template is the integrated proof that earlier phases compose. | One scaffolder run creates a service; CI passes; deploy appears in Argo; signals/policy/catalog are connected. |
| 7.4 Add Crossplane self-service database | 20% | Define XRD, Composition, providers/credentials, allowed sizes/regions, policy and lifecycle; request a database by PR/claim; test deletion and protection. | Self-service requires an API and guardrails, not direct cloud-admin credentials for every developer. | Developer claim provisions permitted DB; status/errors are understandable; forbidden size is rejected; deletion behavior is safe. |
| 7.5 Add scorecards and platform measures | 10% | Score services for ownership, SLO, alerts, security, freshness, and update cadence; choose adoption and time-to-first-deploy measures. | A platform is a product; adoption and outcomes matter more than portal feature count. | Scorecard data is derived from source systems; at least one measure informs a backlog improvement. |
| 7.6 User test, operational handoff, and close | 15% | Ask another person or cold-start yourself after a break to create a service; observe friction without coaching; fix the most important failures; finish ADR/retro/runbooks. | A path is not golden until someone other than its author can use it. | Tester reaches observed deployment; friction items are filed/fixed; phase gate passes. |

### Phase 7 phase gate

- [ ] Every service has an owner and catalog entry.
- [ ] A new service reaches build, policy, deployment, observability, and ownership through
    the documented scaffolder path.
- [ ] A developer can request a database without manual cloud-console work, within policy.
- [ ] A user test changed or validated the platform design.
- [ ] Platform measures, phase ADR, retro, and runbook are complete.

---

## Phase 8+ — Advanced pass (separate from the first-lap 100%)

After Phase 7, revisit the advanced/P2 column in `02_TECH_MATRIX.md`. Track each cycle
with its own 100% denominator; do not change the first-lap score.

| Cycle | Advanced target |
|---|---|
| 8 | Write a Kubernetes operator (kubebuilder) managing Trackline tenants as a CRD |
| 9 | Istio ambient or Linkerd: mTLS, circuit breaking, traffic splitting |
| 10 | Thanos or Mimir: HA, long retention, cardinality governance |
| 11 | Vault PKI/transit/auto-unseal, short-lived certs, SPIFFE/SPIRE |
| 12 | SLSA L3 provenance, policy-controller verification chains, VEX |
| 13 | Cilium/eBPF datapath, Hubble, Tetragon, continuous profiling |
| 14 | Flink state/savepoints/exactly-once semantics and Kafka tiered storage |
| 15 | Multi-cluster/multi-region active-active and global traffic management |

---

## Phase close: required every time

Do these after the technical gate passes and before pulling the next phase's work:

1. **Review evidence:** walk every phase-gate checkbox; link the command, test, dashboard,
  or drill result that proves it.
2. **Update ADO:** close only accepted items; create follow-up work for gaps instead of
  waiving them. Keep parent/child and predecessor links accurate.
3. **Write ADRs:** record consequential choices, alternatives, and downside accepted.
4. **Write the retro/postmortem:** what happened, how it was detected, time to recover,
  what changes in process or system. No blame; no "be more careful" actions.
5. **Update runbooks and diagrams:** anything you had to rediscover gets written down;
  diagrams change in the same change as the architecture.
6. **Recalculate progress:** check the accepted step weights, add the phase contribution,
  and record the new cumulative percentage in ADO or the status block below.
7. **Groom only the next phase:** update its prerequisites and cost assumptions using
  what you learned; leave later-phase detail at roadmap level.

### Progress log

Append one row after each accepted step or phase gate. Do not erase old rows; this gives a
small audit trail of how the project progressed.

| Date | Phase / step | Weight earned | Cumulative first-lap progress | Evidence / ADO item |
|---|---|---:|---:|---|
| 2026-09-29 | Baseline recorded; Linux spike reported complete, repo hookup pending | 0% assigned pending phase-step acceptance | 0% baseline | ADO #28 Closed; #24 Active; see baseline above |

The baseline row intentionally awards no phase percentage yet: the Linux spike is a real
learning milestone, but it is one part of Phase 0's foundation and is not by itself an
accepted roadmap step. Once step 0.2's remaining study checks are satisfied, record its
10% phase-step weight, contributing `14% × 10% = 1.4%` to the first-lap total.

---

## Next action from the current baseline

1. **Verify the public repo boundary (`HEL-0.1.1`, #24):** determine the exact public repo
  URL and ensure it contains only HELIOS. The current workspace remote points to
  `Ferrai/Limitless`; do not push the whole parent `Limitless` workspace. Add the project
  `.gitignore`/`.gitattributes` and verify a fresh clone before publishing application
  source.
2. **Pin the toolchain (`HEL-0.1.2`, #25):** create `.mise.toml` with exact versions for
  Go, Node, Rust, Python, kubectl, Helm, Terraform/OpenTofu, k6, and gitleaks. Run
  `mise install` and `mise doctor`; record versions. This is the first implementation
  task now that the learning is complete.
3. **Add secret scanning (`HEL-0.1.3`, #26):** configure gitleaks pre-commit, scan the
  repository history, plant a harmless test secret to prove detection, then remove it.
  Remember that the hook is convenience; CI becomes the enforcement control later.
4. **Establish the container runtime (`HEL-0.1.4`, #27):** use OrbStack on the Apple
  Silicon Mac, run `docker run --rm hello-world`, confirm buildx supports amd64 and
  arm64, and record idle memory/CPU. Decide which runtime owns the Docker socket before
  removing or retaining Docker Desktop.
5. **Build the reference image (`HEL-0.3.1`, #33):** once #25 and #27 are accepted, use
  the already-completed container spike to build `edge-gateway` as the reference image.
  Do not start the remaining service images before the reference standard passes.

At this point you do **not** need another study spike. You need to convert the knowledge
into repository/toolchain evidence, one small work item at a time.

If #32 or #29 is already complete, record the evidence in ADO and continue with the next
unblocked item. The board is authoritative for item state; this list explains the safe
sequence and must be refreshed when that state changes.
