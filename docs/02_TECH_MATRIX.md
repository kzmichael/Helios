# Technology Matrix

Every technology in HELIOS, the phase it enters, and the depth to reach.

**Depth scale:** L1 Aware · L2 Working · L3 Fluent · L4 Deep (see `00_CHARTER.md` §5)

**Phase 1 rule:** L2 on everything, L3 on the *primary* of each category only.
Resist going deeper early — that is how breadth projects die.

---

## Legend

- **P1** = first pass (breadth). Target depth shown.
- **P2** = advanced revisit pass (Phase 8+).
- **Primary** = the one tool in that category you go L3 on now. Others stay L1/L2.

---

## 1. Operating System & Runtime

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Linux** *(primary)* | **L3** | Diagnose a CPU-throttled, memory-pressured container from inside the node using `ss`, `pidstat`, `strace`, cgroup v2 files | eBPF tracing, kernel tuning, io_uring, `perf` flame graphs |
| systemd | L2 | Write a hardened unit with resource limits + restart policy for the on-prem tier | Socket activation, slices, systemd-nspawn |
| Bash | L3 | Write a `set -euo pipefail` script with traps, arg parsing, and idempotency | — |
| Networking (kernel) | L2 | Explain a packet's path through iptables/nftables into a pod | Cilium/eBPF datapath, XDP |

## 2. Containers

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Docker / OCI** *(primary)* | **L3** | Multi-stage, distroless, non-root, `<50 MB` image with BuildKit cache mounts, reproducible | Rootless, OCI spec internals, image layer surgery |
| Compose | L2 | Full local stack before k8s exists | — |
| BuildKit / buildx | L2 | Multi-arch (amd64+arm64) builds from an arm64 Mac | Remote cache, custom frontends |
| Alternatives | L1 | Know when Podman / Kaniko / ko / Buildpacks beat Docker | Build one service with each |

## 3. Orchestration

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Kubernetes** *(primary)* | **L3** | Deploy all of Trackline with probes, resource requests/limits, PDBs, anti-affinity, RBAC, NetworkPolicy — and debug a CrashLoopBackOff, a pending pod, and a failing readiness probe from first principles | Operators (kubebuilder), CRDs, scheduler tuning, admission webhooks, cluster-api, multi-cluster |
| Helm | L2 | Chart with values per env, hooks, and a tested upgrade path | Library charts, `helm test`, chart signing |
| Kustomize | L2 | base + 3 overlays, no duplication | Components, generators, transformers |
| k3d / kind | L2 | Reproducible local cluster from one command | — |
| KEDA | L2 | Scale `ingest` on Kafka consumer lag | Custom scalers |
| Karpenter / Cluster Autoscaler | L1→L2 | Nodes appear under load, disappear after | Consolidation, spot interruption handling |

## 4. Infrastructure as Code

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Terraform / OpenTofu** *(primary)* | **L3** | Reusable modules, remote state + locking, `dev/stage/prod` live dirs, plan gated in CI, full `apply`→`destroy` cycle nightly | Write a custom provider, Terragrunt, state surgery, import existing infra, Sentinel/OPA policy |
| Bicep | L2 | Provision the Azure prod tier | ARM template internals, deployment stacks |
| Pulumi | L1→L2 | Provision one component (Cloudflare) in TypeScript | Automation API, dynamic providers |
| CloudFormation / CDK | L1 | Read and explain one; know why you didn't pick it | Build one stack with CDK |
| **Crossplane** | L1 | Understand the control-plane-as-IaC model | **P2 headline: compositions become the self-service layer of the IDP** |

## 5. Configuration Management

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Ansible** *(primary)* | **L2→L3** | Roles + inventory that build the on-prem k3s tier from bare VMs, idempotent on re-run, tested with Molecule | Collections, custom modules, AWX/Tower, dynamic inventory, Ansible Vault |
| cloud-init | L2 | VM bootstraps itself into the cluster with no manual step | — |
| Packer | L1→L2 | Golden image for node pools | Image pipelines + CVE-driven rebuilds |
| Salt / Chef / Puppet | L1 | Explain push vs pull and when each wins | Build one role in Salt for comparison |

## 6. CI (Build)

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **GitHub Actions** *(primary)* | **L3** | Matrix builds, reusable workflows, OIDC to cloud (zero static keys), caching, path filters, required checks | Self-hosted runners on k8s (ARC), composite actions, workflow security hardening |
| GitLab CI | L2 | Mirror one service's pipeline; compare DAG + caching models | Runners, parent-child pipelines |
| Jenkins | L2 | Declarative pipeline + shared library for one service | JCasC, dynamic k8s agents |
| Azure DevOps | L2 | Pipeline for the Azure prod tier (uses your MSDN) | Environments, approvals, YAML templates |
| Dagger / Earthly | L1 | Know the "CI as code you can run locally" model | Port one pipeline |

## 7. CD (Deploy)

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Argo CD** *(primary)* | **L3** | App-of-apps, sync waves, drift detection + auto-heal, multi-cluster targets, RBAC per team | ApplicationSets, multi-tenancy, notifications, custom health checks |
| Flux | L2 | Manage one cluster with Flux to feel the difference | Image automation, OCI artifacts |
| **Argo Rollouts / Flagger** | L2 | Canary with automated rollback driven by a Prometheus SLO query | Experiments, traffic mirroring, mesh-driven analysis |
| Feature flags (OpenFeature / Unleash) | L2 | Decouple deploy from release for one risky feature | Progressive rollout by tenant tier |

## 8. Cloud Platforms

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **AWS** *(primary)* | **L3** | VPC + subnets + NAT, EKS, RDS, S3, IAM/IRSA — all in Terraform, all destroyable. **Kafka self-hosted via Strimzi, never MSK** (see cost model) | Organizations, landing zone, SCPs, Transit Gateway, Well-Architected review |
| Azure | L2 | AKS + Entra ID + Azure Monitor prod tier via Bicep | Landing zones, Azure Policy, Private Link |
| GCP | L2 | GKE + Cloud Run + BigQuery DR/analytics tier | Workload Identity Federation, VPC-SC, Anthos |
| Cloudflare | L2 | DNS, WAF, CDN, R2, Tunnel, one Worker | Durable Objects, Load Balancing, Zero Trust Access |

## 9. Networking & Service Mesh

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| DNS | L3 | Delegate a subdomain per env; explain a full resolution path incl. caching failure | GeoDNS, anycast, DNSSEC |
| TLS / PKI | L3 | cert-manager + ACME, automatic rotation, internal CA for mTLS | Vault PKI engine, short-lived certs, SPIFFE IDs |
| Ingress / Gateway API | L2→L3 | Migrate from Ingress to Gateway API; explain why | Custom filters, multi-gateway topologies |
| **Service mesh** | L1 | Know sidecar vs ambient vs sidecar-less tradeoffs | **P2 headline: Istio ambient or Linkerd — mTLS, retries, circuit breaking, traffic splitting** |
| Cilium / eBPF | L1 | Understand what replaces kube-proxy and why | Network policy at L7, Hubble, cluster mesh |
| Global traffic mgmt | L1 | — | Health-based failover AWS↔Azure↔GCP via Cloudflare LB |

## 10. Observability

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **OpenTelemetry** *(primary)* | **L3** | All services auto+manual instrumented; Collector with processors/exporters; trace spans a device→map request across 4 services | Tail sampling, custom processors, eBPF auto-instrumentation, OTTL |
| Prometheus | L3 | RED + USE dashboards, recording rules, exemplars linking metrics→traces | Thanos or Mimir for HA + long retention, federation, cardinality control |
| Grafana | L3 | Dashboards as code, provisioned, versioned in Git | Scenes, plugins, SLO app |
| Loki | L2 | Structured logs correlated to trace IDs | Log pipelines with Vector/Fluent Bit, retention tiering |
| Tempo | L2 | Trace → logs → metrics navigation working in one click | Tail sampling, TraceQL, service graphs |
| Grafana Cloud | L2 | Second backend, free tier — prove OTel vendor-neutrality | — |
| Azure Monitor / CloudWatch | L2 | Third + fourth backend, same OTel pipeline | Log Analytics KQL, CloudWatch Insights |
| **Profiling (Pyroscope)** | L1 | — | **P2: continuous profiling, flame graphs, find the real hot path** |
| RUM + synthetics | L1 | — | **P2: real user monitoring + synthetic checks from 3 regions** |

## 11. SLO & Incident Management

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **SLO engineering** *(primary)* | **L3** | 5 SLOs defined as code (Sloth/Pyrra), multi-window multi-burn-rate alerts, written error budget policy | Per-tenant SLOs, dependency SLOs, SLO-gated deploys |
| Alertmanager | L2→L3 | Routing, grouping, inhibition, silences — zero noisy alerts | Alert fatigue metrics, alert review ritual |
| Grafana OnCall / PagerDuty | L2 | Real rotation, real escalation, you carry the pager | Auto-remediation, ChatOps runbook execution |
| Runbooks | L3 | Every alert links to a runbook that a stranger could execute | Executable runbooks |
| Postmortems | L3 | 5 blameless postmortems with tracked action items | Incident metrics (MTTD/MTTA/MTTR), review board |

## 12. Secrets & Identity

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **OIDC / workload identity** *(primary)* | **L3** | Zero long-lived cloud keys anywhere — CI, pods, and humans all use federated short-lived credentials | Cross-cloud federation, SPIFFE/SPIRE |
| External Secrets Operator | L2 | Secrets sync from cloud KMS into k8s, rotated | Push secrets, generators |
| **Vault** | L1 | Understand dynamic secrets vs static | **P2 headline: dynamic Postgres creds, PKI engine, transit encryption, auto-unseal** |
| SOPS / age | L2 | Encrypted secrets safely in Git for GitOps | KMS-backed keys |

## 13. Security & Supply Chain

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| Trivy / Grype | L2 | Image + IaC + filesystem scanning as a CI gate with a documented exception process | VEX, reachability analysis |
| gitleaks | L2 | Pre-commit + CI secret scanning; one planted secret caught | — |
| **Cosign / Sigstore** | L2 | All images signed keylessly; SBOM attached as attestation | SLSA L3 provenance, policy-controller verification chains |
| **Kyverno or OPA Gatekeeper** *(primary)* | **L2→L3** | Admission rules: no `:latest`, no root, no unsigned images, required labels — with a demonstrated rejection | Mutating policies, OPA/Rego for non-k8s, Conftest for IaC |
| Checkov / tfsec | L2 | IaC misconfig gate in PRs | Custom policies |
| Falco / Tetragon | L1 | — | **P2: runtime detection rules, response automation** |
| SIEM (Wazuh / Elastic) | L1 | — | **P2: audit logs → SIEM, detections, correlation rules** |
| CSPM (Prowler / ScoutSuite) | L1 | One baseline scan, findings triaged | Continuous posture, CIS benchmark compliance |
| Threat modelling | L1 | One STRIDE pass on `edge-gateway` | Full-system model, attack trees |

## 14. Data Operations

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **PostgreSQL ops** *(primary)* | **L2→L3** | Backup + **verified restore drill**, connection pooling, `EXPLAIN ANALYZE` on the slow query you find under load | PITR, logical replication, failover, zero-downtime schema change, `pg_stat_statements` tuning |
| Migrations (Atlas / Flyway) | L2 | Migrations run in CD, reversible, never block a deploy | Expand/contract pattern, online DDL |
| Kafka ops | L2 | Partition strategy, consumer lag dashboards, retention policy | Rebalance tuning, tiered storage, Schema Registry compat, MirrorMaker |
| Redis ops | L2 | Eviction policy chosen deliberately; stampede reproduced and fixed | Cluster mode, persistence tradeoffs |
| Backup/DR tooling (Velero) | L2 | Cluster state backup + restore into a *fresh* cluster | Cross-region/cross-cloud restore |

## 15. Resilience & Performance

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **k6** *(primary)* | **L2→L3** | Load profiles that reproduce each seeded weakness; thresholds as CI gates | Distributed k6 on k8s, soak + spike + breakpoint tests |
| Chaos (Litmus / Chaos Mesh) | L2 | Kill pods, inject latency, partition network — with a hypothesis written *before* | AWS FIS, region failover drill, dependency isolation |
| Game days | L2 | One facilitated game day with an incident commander | Quarterly cadence, unannounced drills |
| pprof / profiling | L2 | Find a hot path in `ingest` under load | Continuous profiling, allocation analysis |
| Capacity planning | L1→L2 | Headroom model: "we can serve N devices on M nodes" | Little's Law, queueing theory, forecast-driven scaling |

## 16. FinOps

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| Tagging strategy | L2 | Every resource tagged: env, owner, tenant-tier, cost-center — enforced by policy | — |
| Infracost | L2 | Cost delta comment on every IaC PR | Policies that block expensive changes |
| Budgets + alerts | L2 | Hard budget alarms on all three clouds | Anomaly detection |
| OpenCost / Kubecost | L1 | — | **P2: cost per tenant per namespace on a dashboard** |
| Rightsizing / spot / ARM | L2 | Workloads on spot + Graviton/arm64 with interruption handling | Commitment planning, VPA-driven rightsizing |

## 17. Platform Engineering

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Preview environments** *(primary)* | **L2→L3** | Every PR gets an ephemeral namespace + URL, torn down on merge | Full-stack previews with seeded data |
| Backstage | L1 | — | **P2 headline: service catalog, ownership, TechDocs, scorecards, scaffolder golden path** |
| Golden paths | L1 | — | **P2: one PR scaffolds a new service with CI, CD, o11y, SLOs, and policies attached** |
| Devcontainers | L2 | Reproducible dev env, one-click | — |
| Tilt / Skaffold | L2 | Hot reload against local k3d | — |
| mise / asdf | L2 | Pinned toolchain versions, identical in CI and local | — |

## 18. Programming

| Tech | P1 depth | P1 proof-of-competence | P2 |
|---|---|---|---|
| **Python** *(primary for automation)* | **L3** | Write the platform CLI + a non-trivial automation (cost report, SLO reporter) | Async, packaging, testing discipline |
| Go | L2 | Read and modify the Go services; write a small k8s-aware tool | **P2: write a Kubernetes operator and a production CLI** |
| SQL | L2→L3 | Read execution plans, fix the slow query | Window functions, CTE optimization |
| YAML/JSON/Jsonnet/CUE | L2 | Know when templating breaks down | CUE or Jsonnet for config correctness |

---

## Anti-scope (deliberately excluded)

| Excluded | Why |
|---|---|
| Multiple monitoring *agents* per signal | Instrument once with OTel, fan out to backends. Installing Datadog+New Relic+Dynatrace agents teaches nothing but installation. |
| Every cloud service | Only services the app needs. No "let's add SQS because it exists." |
| Mesos / Nomad / Swarm | k8s won. L1 awareness only. |
| Building your own CI system | Not a skill gap. |
