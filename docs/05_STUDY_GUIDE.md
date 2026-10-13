# Study Guide

Companion to `02_TECH_MATRIX.md`. The matrix says *which* technologies and *how deep*.
This says **which specific topics to study**, in order, per phase.

---

## How to use this

1. **Study just-in-time, not up front.** Do not read this whole document and try to learn
   it before building. Take one phase's list, study it, build the phase, move on. Knowledge
   without an immediate application evaporates.
2. **Each phase lists topics, then self-check questions.** If you cannot answer the
   self-checks from memory, you are not done — regardless of whether the phase "works".
3. **Existing notes live in `Learn-Better/`.** Cross-references are given where your
   folders already cover a topic; use them, extend them, don't restart them.
4. **"Know" means L2 unless marked otherwise** (L1 Aware · L2 Working · L3 Fluent · L4 Deep).

---

# Phase 0 — Foundations & Application

## Linux internals — target L3
*(`Learn-Better/01_Linux_System_Engineering/`)*

- **Processes:** fork/exec, PID namespaces, zombie vs orphan, signals (SIGTERM vs SIGKILL),
  process states in `ps`, what PID 1 does and why it matters in a container
- **cgroups v2:** CPU shares vs quota vs throttling, memory limits vs requests, the OOM
  killer's decision process, where the counters live in `/sys/fs/cgroup`
- **Namespaces:** the six types (pid, net, mnt, uts, ipc, user) — what each isolates
- **Filesystems:** inodes, hard vs symbolic links, mount propagation, overlayfs (this *is*
  how container layers work), `df` vs `du` disagreement and why
- **Tools to be fluent with:** `ps`, `top`/`htop`, `ss`, `lsof`, `strace`, `dmesg`,
  `journalctl`, `iostat`, `vmstat`, `pidstat`
- **File descriptors:** limits, leaks, `ulimit`, why "too many open files" happens

## systemd
*(`Learn-Better/01_Linux_System_Engineering/02_Systemd_Services/`)*

- Unit types, dependency ordering (`After=` vs `Requires=` — these are different axes)
- Restart policies and rate limiting
- Resource limits in units (`MemoryMax`, `CPUQuota`) — the same cgroups, different door
- `journalctl` filtering, log persistence

## Containers — target L3

- **Images:** layers, the union filesystem, layer caching, **cache invalidation order**
  (why `COPY go.mod` before `COPY .` changes everything)
- **Multi-stage builds:** build stage vs runtime stage, what to carry forward
- **Base images:** scratch vs distroless vs alpine vs debian-slim — the musl/glibc problem,
  why alpine can be slower for some workloads
- **Security:** non-root users, read-only root filesystem, dropped capabilities, why a
  shell in a production image is a liability
- **Multi-arch:** `buildx`, manifest lists, why this matters on your arm64 Mac
- **Runtime:** ENTRYPOINT vs CMD, signal handling and PID 1, graceful shutdown
- **Storage:** volumes vs bind mounts vs tmpfs
- **Networking:** bridge networks, DNS between containers, published vs exposed ports

## Bash — target L3

- `set -euo pipefail` and what each flag actually prevents
- Quoting rules — `"$var"` vs `$var` vs `"${arr[@]}"`
- Traps and cleanup, exit codes, `$?`
- Process substitution, here-docs, pipefail behaviour in pipelines
- Writing idempotent scripts (safe to re-run)

## PKI fundamentals — target L2

- Asymmetric crypto: what the public and private key each do
- Certificate chains: root CA → intermediate → leaf, and how trust is established
- X.509 fields that matter: CN vs **SAN** (only SAN is used now), key usage, validity
- CSRs: what's in one, who signs it
- Why your browser trusts anything at all — the OS/browser trust store
- Self-signed vs private CA vs public CA

## Git & repo hygiene

- Branching model, conventional commits, why linear history helps CD
- `.gitignore` vs `.gitattributes`
- Pre-commit hooks, and why a hook is not a security control (it's client-side)
- What must *never* be committed: state files, `.tfvars`, kubeconfigs, `.env`

### Self-check — Phase 0

- [ ] Why does a container die when its main process exits, and what is PID 1's job?
- [ ] Your container was OOM-killed. Walk through how you'd prove it and find the cause.
- [ ] Why does reordering two lines in a Dockerfile change build time by 10×?
- [ ] Draw the trust chain from your browser to a site's certificate.
- [ ] `df` says the disk is full, `du` says it isn't. What's happening?

---

# Phase 1 — First Production

## Networking — target L3
*(`Learn-Better/02_Networking_DevOps/`)*

- **CIDR & subnetting:** compute ranges by hand, plan a non-overlapping VPC address space
  (overlapping CIDRs will block VPC peering later — plan now)
- **The OSI layers that matter:** L3 vs L4 vs L7 load balancing, and when each is correct
- **TCP:** handshake, connection states (`TIME_WAIT` matters), keepalives, backlog queues
- **DNS resolution path:** stub resolver → recursive → root → TLD → authoritative
- **Record types:** A, AAAA, CNAME, TXT, MX, SRV, CAA, and the apex-CNAME problem
- **TTL strategy:** why you lower TTLs *before* a migration
- **NAT:** SNAT vs DNAT, why NAT Gateways cost money, why pods need egress NAT
- **TLS handshake:** SNI, ALPN, session resumption, where certificates fit

## Kubernetes — target L2→L3
*(`Learn-Better/04_Containers_Kubernetes/`)*

- **Architecture:** API server, etcd, scheduler, controller-manager, kubelet, kube-proxy —
  and what fails when each is down
- **The reconciliation loop** — the single most important concept in Kubernetes
- **Workloads:** Pod, ReplicaSet, Deployment, StatefulSet, DaemonSet, Job, CronJob —
  and specifically *when* StatefulSet is required
- **Scheduling:** requests vs limits, QoS classes (Guaranteed/Burstable/BestEffort),
  node selectors, affinity/anti-affinity, taints and tolerations
- **Networking:** ClusterIP vs NodePort vs LoadBalancer, Services vs Endpoints,
  cluster DNS, Ingress vs Gateway API
- **Health:** liveness vs readiness vs startup probes — and the damage a wrong liveness
  probe does under load
- **Config:** ConfigMaps, Secrets (base64 is *not* encryption), env vs volume mounts
- **RBAC:** Role/ClusterRole, bindings, ServiceAccounts
- **Disruption:** PodDisruptionBudgets, graceful termination, `terminationGracePeriodSeconds`
- **Debugging:** `describe`, events, `logs --previous`, `exec`, ephemeral containers

## Terraform — target L3
*(`Training/Terraform/`)*

- **Core model:** desired state, the dependency graph, plan vs apply
- **State:** what's in it, why it's sensitive, remote backends, **locking**, drift
- **Modules:** inputs/outputs, composition, versioning, when *not* to abstract
- **Expressions:** `for_each` vs `count` (and why `for_each` is almost always right),
  locals, dynamic blocks
- **Lifecycle:** `create_before_destroy`, `prevent_destroy`, `ignore_changes`
- **Data sources vs resources**, `depends_on` and implicit dependencies
- **Environment separation:** workspaces vs directories — know the tradeoff, pick one
- **Secrets in Terraform:** why they end up in state, and what to do about it

## CI/CD & GitHub Actions — target L3
*(`Learn-Better/05_CICD_Release_Engineering/`)*

- Workflow/job/step model, runners, matrix builds
- Caching: what to cache, cache keys, why a bad key is worse than no cache
- **OIDC federation to cloud providers** — the whole point; understand the token exchange
- Secrets and variables, environment protection rules
- Reusable workflows vs composite actions
- **Supply-chain hygiene:** pinning actions to SHA, `permissions:` least privilege,
  the risk of `pull_request_target`

## TLS in practice

- ACME protocol: account, order, challenge, finalize
- **HTTP-01 vs DNS-01** — and why wildcards *require* DNS-01
- cert-manager objects: Issuer/ClusterIssuer, Certificate, CertificateRequest, Order
- Renewal timing, and what actually breaks when a cert expires
- CAA records

## Azure — target L2
*(`Ms/`, `Learn/`)*

- Tenant → subscription → resource group hierarchy
- Entra ID: users, groups, service principals, **managed identities**
- RBAC scopes and inheritance
- AKS: node pools, spot nodes, the free control-plane tier, CNI choice
- Networking: VNet, subnet delegation, NSGs
- Cost management: budgets, alerts, tags

## Prometheus — target L2

- Pull model, scrape configs, service discovery, relabelling
- Metric types: counter, gauge, histogram, summary — and when each is correct
- **PromQL:** selectors, `rate()` vs `increase()`, aggregation, `by`/`without`, histogram
  quantiles
- **Cardinality** — what explodes it, why it kills Prometheus
- Recording rules vs alerting rules

### Self-check — Phase 1

- [ ] A pod is `Pending`. List every possible cause and how you'd distinguish them.
- [ ] Why does a misconfigured liveness probe make an overloaded service *worse*?
- [ ] Explain the OIDC exchange that lets GitHub Actions deploy without stored keys.
- [ ] Why must wildcard certificates use DNS-01?
- [ ] Two engineers run `terraform apply` simultaneously. What prevents corruption?
- [ ] Why is `rate()` over a counter correct but `rate()` over a gauge meaningless?

---

# Phase 2 — GitOps & Progressive Delivery

## GitOps model
- Push vs pull deployment, and why pull is more secure (no cluster credentials in CI)
- Reconciliation, drift detection, self-healing
- Repository topology: mono vs poly, app repo vs config repo, and the image-update problem
- Argo CD objects: Application, Project, ApplicationSet, sync waves, hooks, health checks

## Manifest management
- Kustomize: bases, overlays, patches (strategic merge vs JSON6902), generators
- Helm: templating, values precedence, hooks, release lifecycle, `--atomic`
- **When templating becomes a liability** — the argument for Kustomize/CUE

## Progressive delivery
- Blue/green vs canary vs rolling — failure and rollback characteristics of each
- Analysis-driven promotion: which metrics gate a canary, and over what window
- **Deploy vs release** — feature flags decouple them
- Traffic shifting mechanisms: replica-count vs mesh-weighted

## Database migrations
- Expand/contract (parallel change) — the only safe pattern for zero downtime
- Why migrations must be backward-compatible with the *previous* app version
- Locking behaviour of DDL in Postgres, and which operations block writes
- Forward-only vs reversible migrations

## Secrets in GitOps
- The bootstrap problem: how does the cluster get its first secret?
- SOPS/age vs Sealed Secrets vs External Secrets — the threat model each addresses

### Self-check — Phase 2
- [ ] Why does pull-based CD reduce blast radius versus push?
- [ ] Your canary looks healthy for 5 minutes then fails. What analysis window was wrong?
- [ ] Describe deploying a column rename with zero downtime.
- [ ] Someone `kubectl edit`s prod. What happens, and how fast?

---

# Phase 3 — Observability, SLOs & On-Call

## Observability theory
- Monitoring vs observability; known-unknowns vs unknown-unknowns
- **The signals:** metrics, logs, traces, profiles, events — cost and resolution of each
- **RED** (Rate, Errors, Duration) for services, **USE** (Utilization, Saturation, Errors)
  for resources — know which applies where
- Cardinality and sampling as cost-control mechanisms

## OpenTelemetry — target L3
- Spec vs SDK vs Collector vs protocol (OTLP)
- **Context propagation** — trace context headers, and why it breaks across async boundaries
- Spans, attributes, events, links; parent/child vs follows-from
- Collector pipelines: receivers → processors → exporters
- Head vs tail sampling — and why tail sampling needs the whole trace
- Semantic conventions (use them; custom attribute names destroy portability)
- **Correlation:** exemplars (metrics→traces), trace_id in logs (logs→traces)

## SLO engineering — target L3
- SLI vs SLO vs SLA; why you never publish your internal SLO
- Choosing SLIs: user-journey-based, not resource-based
- Error budgets, budget policy, and what it must *forbid* when exhausted
- **Multi-window multi-burn-rate alerting** — why single-threshold alerts are either
  too slow or too noisy
- Request-based vs time-window-based SLIs
- The "100% is the wrong target" argument

## Alerting & on-call
- Symptom-based vs cause-based alerting (alert on the former)
- Page vs ticket vs dashboard — the three tiers
- Alertmanager: routing trees, grouping, inhibition, silences
- Alert fatigue as a measurable failure of the alerting system
- Incident roles: Incident Commander, Ops Lead, Comms Lead
- Blameless postmortems, contributing factors vs root cause, action item tracking
- MTTD / MTTA / MTTR and what each actually tells you

## Logging
- Structured logging, log levels that mean something, correlation IDs
- Log pipelines: agent → buffer → backend, and backpressure in that chain
- Retention tiering and cost
- **What must never be logged:** PII, secrets, full request bodies

### Self-check — Phase 3
- [ ] Choose an SLI for "the live map is useful". Defend it.
- [ ] Why do multi-burn-rate alerts use two windows?
- [ ] A trace is missing its downstream spans. List the likely causes.
- [ ] Your error budget is exhausted. What are you now forbidden from doing?
- [ ] Which is worse: a page that fires with no action needed, or one that never fires?

---

# Phase 4 — Security & Supply Chain
*(`Learn-Better/08_Security_DevSecOps/`)*

## Identity
- Authentication vs authorization vs accounting
- OAuth2 vs OIDC — what each actually is (one is authz, one is authn on top)
- JWT structure, signature validation, expiry, **why you must validate `aud` and `iss`**
- Workload identity: IRSA, Azure Managed Identity, GCP Workload Identity Federation
- SPIFFE/SPIRE concepts: workload attestation, SVIDs
- Why long-lived credentials are the root of most cloud breaches

## Secrets
- Secret lifecycle: generation, distribution, rotation, revocation, audit
- Static vs **dynamic** secrets (Vault's key idea)
- Envelope encryption and KMS
- Why Kubernetes Secrets are base64, not encrypted — and how to fix it (etcd encryption at rest)

## Supply chain
- The threat model: dependency confusion, typosquatting, compromised build systems
- **SBOM** formats (SPDX, CycloneDX) and what they're actually for
- Signing: keyless signing, Fulcio, Rekor transparency log, how cosign verification works
- **SLSA** levels — what provenance proves and what it doesn't
- Admission-time verification: policy-controller, Kyverno image verification

## Policy as code
- OPA/Rego basics; Gatekeeper vs Kyverno (Rego vs YAML — know the tradeoff)
- Validating vs mutating vs generating policies
- Admission control flow: authentication → authorization → mutating → validating → etcd
- Policy exceptions, and why they must expire

## Kubernetes security
- Pod Security Standards (privileged/baseline/restricted)
- Capabilities, seccomp, AppArmor, read-only root filesystem
- **NetworkPolicy** — default-deny, ingress vs egress rules, why it needs CNI support
- The four attack paths: container escape, credential theft, supply chain, misconfiguration
- Audit logging: what to capture, where to send it

## Threat modelling
- STRIDE categories
- Trust boundaries and data flow diagrams
- Risk = likelihood × impact; why you rank rather than fix everything

### Self-check — Phase 4
- [ ] Explain keyless signing without using the word "magic".
- [ ] What does SLSA provenance prove, and what can it still not prevent?
- [ ] Someone gets read access to etcd. What do they now have?
- [ ] Why is a pre-commit secret-scanning hook not a security control?
- [ ] Draw the trust boundaries around `edge-gateway`.

---

# Phase 5 — Multi-Cloud, Multi-IaC & Config Management

## AWS — target L3
*(`Learn/AWS/`, `Learn-Better/03_Cloud_AWS/`)*
- IAM: policies, roles, trust policies, assume-role, permission boundaries, policy evaluation order
- VPC: subnets, route tables, IGW vs NAT GW, security groups (stateful) vs NACLs (stateless)
- EKS: control plane, node groups, **IRSA**, the aws-auth/access-entry model, CNI IP exhaustion
- S3: storage classes, lifecycle, versioning, bucket policies vs IAM, encryption options
- RDS: parameter groups, multi-AZ vs read replicas, backup windows, PITR
- Cost mechanics: what NAT, cross-AZ, and egress actually charge for

## Ansible — target L2→L3
*(`Training/Ansible/`)*
- Inventory (static/dynamic), groups, host/group vars precedence
- Playbooks, roles, handlers, tags
- **Idempotency** — the defining property; how modules achieve it; why `shell`/`command` break it
- Templating with Jinja2, facts, `register`, conditionals, loops
- Vault for secrets
- Molecule for role testing
- Push vs pull models; where Ansible stops and IaC begins

## Multi-cloud concepts
- Workload placement: data gravity, latency, egress cost, compliance
- Identity federation across providers
- Global traffic management: GeoDNS, anycast, health-check-driven failover
- The portability myth — what actually ports and what doesn't

## Hybrid networking
- Site-to-site VPN vs private interconnect vs tunnel-based (Cloudflare Tunnel)
- Overlapping CIDR problems (this is why you planned addressing in Phase 1)
- Split-horizon DNS

### Self-check — Phase 5
- [ ] Explain IRSA end to end: pod → token → role → AWS API call.
- [ ] Security group vs NACL — give a case where only one works.
- [ ] Why is an Ansible task using `shell` usually a defect?
- [ ] Your AWS bill shows large "data transfer" charges with no internet traffic. Why?

---

# Phase 6 — Resilience, Scale & FinOps
*(`Learn-Better/12_Incident_Management_SRE/`)*

## Performance & capacity
- **Latency percentiles** — why averages lie, why p99 is a user, why you can't average percentiles
- Little's Law: `L = λW` — and using it to size queues and pools
- Queueing behaviour near saturation (the hockey stick)
- Coordinated omission in load testing — why naive tools understate latency
- Load test types: smoke, load, stress, soak, spike, breakpoint
- Bottleneck identification: CPU vs memory vs IO vs lock vs network

## Resilience patterns
- Timeouts, retries, **exponential backoff with jitter** (and why jitter is mandatory)
- Retry amplification — how retries cause the outage they're recovering from
- Circuit breakers, bulkheads, load shedding, graceful degradation
- Idempotency keys and exactly-once delivery as a fiction
- Cascading failure and the thundering herd
- Cache patterns: stampede, negative caching, TTL jitter

## Chaos engineering
- Steady-state hypothesis, blast radius, abort conditions
- Why a hypothesis must be written *before* the experiment
- Game day structure and roles

## Backup & DR
- **RTO vs RPO** — define both numerically before designing anything
- Backup types: full/incremental/differential, snapshot vs logical
- **A backup that has never been restored is not a backup**
- PITR and WAL archiving
- DR strategies: backup/restore, pilot light, warm standby, active/active — cost vs RTO

## Autoscaling
- HPA (metrics, stabilization windows, thrashing), VPA, Cluster Autoscaler vs Karpenter
- KEDA: event-driven scaling, scaling on queue depth/lag, scale-to-zero
- Why scaling on CPU is usually the wrong signal
- Spot interruption handling

## FinOps
- Tagging/allocation, showback vs chargeback
- Unit economics: cost per tenant, per request, per GB ingested
- Commitment models (reserved/savings plans) vs spot vs on-demand
- Rightsizing from actual utilization data

### Self-check — Phase 6
- [ ] Why can't you average two p99 values?
- [ ] How do retries turn a partial failure into a total one?
- [ ] State Trackline's RTO and RPO, and justify the numbers.
- [ ] Why is scaling `ingest` on CPU wrong? What's the right signal?
- [ ] Your restore drill takes 4 hours. Your RTO is 1 hour. What are your options?

---

# Phase 7 — Platform Engineering

- Platform as a **product**: users, adoption metrics, feedback loops
- Golden paths vs golden cages — paved road, not mandated road
- Cognitive load as the metric platform teams optimize (Team Topologies)
- Self-service interfaces: API vs CLI vs portal vs PR
- Service catalog, ownership, dependency graphs
- Crossplane model: control plane, providers, compositions, claims
- Scorecards and maturity models
- Ephemeral environments: data seeding, cost control, teardown guarantees
- DORA metrics: deploy frequency, lead time, change fail rate, MTTR — and their misuse

### Self-check — Phase 7
- [ ] Why is a mandatory platform a failed platform?
- [ ] A developer needs a database. Design the self-service path.
- [ ] Which DORA metric is easiest to game, and how?

---

# Continuous — study throughout, never "finished"

- **Distributed systems:** CAP and its misreadings, consistency models, consensus (Raft),
  idempotency, clock skew, the fallacies of distributed computing
- **System design:** `Learn-Better/11_System_Design_DevOps/`
- **Databases:** indexing, query plans, isolation levels, locking, connection pooling —
  `Learn-Better/09_Databases_Caching/`
- **Programming:** Python for automation, Go for reading and writing cloud-native tooling
- **Communication:** ADRs, postmortems, design docs — `Learn-Better/13_Behavioral_Soft_Skills/`

---

## The rule that matters most

For every topic above, the test is not "have I read about it" but:

> **Can I explain it to someone else, and can I debug it when it breaks at 2am?**

If the answer is no, you are at L1 regardless of how much you've read. Build the thing,
break the thing, fix the thing. That is the only path to L2.
