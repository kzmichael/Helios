# Environments, Accounts & Cost Strategy

> Constraint: **~$0–15/mo out of pocket**, plus a renewing **$50/mo MSDN credit**.
> Target: as close to production reality as that money allows.

The constraint is treated as a *feature*. Ephemeral-by-default infrastructure forces IaC
discipline that a generous budget would let you skip.

---

## 1. Account inventory

| Account | Cost | Role in HELIOS | Notes |
|---|---|---|---|
| **Azure (MSDN, $50/mo renewing)** | $0 net | **Always-on production tier** | Most valuable asset because it *recurs*. Credit exhaustion disables the subscription rather than billing you — a free hard budget guardrail. |
| **AWS (new account)** | ~$0–5 | **Ephemeral deep-dive cloud** | 12-month free tier. EKS control plane is **$0.10/hr = $73/mo — never leave it running.** |
| **GCP (new account)** | $0 | **DR + ML + analytics tier** | $300 / 90-day credit. **Create this account in Phase 4, not now** — the 90-day clock starts at signup. |
| **Cloudflare (free)** | $0 | **Permanent edge tier** | DNS, WAF, CDN, R2 (10 GB), Workers, Tunnel, Zero Trust. Genuinely production-grade at $0. |
| **GitHub (public repo)** | $0 | SCM, CI, registry, IaC state backup | **Public repo = unlimited Actions minutes + free GHCR.** Private caps at 2000 min/mo. |
| **Grafana Cloud (free)** | $0 | Second observability backend | 10k metrics / 50 GB logs / 50 GB traces / 500 profiles. Proves OTel vendor portability. |
| **Local Mac** (16 GB, arm64) | $0 | Dev + simulated "on-prem datacenter" | k3d via OrbStack. Binding constraint is RAM. |
| **KodeKloud** | owned | **Drills only — never project infra** | Ephemeral, no persistent state, no multi-day GitOps. Learn a tool there, build it for real here. |
| **`kshitizmichael.xyz`** | **~$12/yr** | **The lab domain** — all HELIOS environments | Deliberately *not* the personal-identity domain. See §9. |
| `kshitizmichael.in` | ~$12/yr | Future personal site — **not yet purchased** | Bought once domain/DNS/email operations are well understood. Kept clean of lab experiments. |

**Estimated real spend: $0–15/month.**

---

## 2. Environment topology

| Env | Where | Lifetime | Purpose |
|---|---|---|---|
| `local` | Mac — k3d via OrbStack | On demand | Inner dev loop; Tilt hot-reload |
| `preview-*` | Azure AKS, per-PR namespace | Life of the PR | Platform-engineering deliverable |
| `dev` | Azure AKS | Always on | Integration, first GitOps target |
| `stage` | Azure AKS (separate node pool) | Always on | Pre-prod, load tests, chaos experiments |
| `prod` | Azure AKS | **Always on — the SLO target** | Real users (simulated), real on-call |
| `onprem` | Mac k3d + Cloudflare Tunnel | Always on | Ansible target, bare-metal ops, hybrid networking |
| `aws-lab` | AWS EKS + VPC | **Ephemeral — destroyed nightly** | Terraform mastery, AWS depth, DR target |
| `gcp-dr` | GCP GKE Autopilot | Phase 5 window only | Cross-cloud failover, ML, analytics |

---

## 3. Why Azure is prod and AWS is not

Counter-intuitive, but correct under this budget:

- AKS **control plane is free**; EKS control plane is **$73/mo**. With a $50/mo renewing
  credit, Azure is the only cloud that can host an always-on cluster sustainably.
- An always-on prod is **required** for SRE work: SLOs need 30-day windows, on-call needs
  something to page about, error budgets need history. You cannot do SRE on infra that
  only exists for two hours.
- AWS remains the cloud you go **L3** on — but as an ephemeral lab. This is *better*
  training: a cluster that is destroyed every night makes click-ops impossible. Every
  change must live in Terraform or it does not survive the night.

**AWS always-on allowance (free tier, no cluster):** S3, Lambda, DynamoDB, CloudWatch,
Route 53 (~$0.50/mo/zone), IAM, ECR. The stateful backbone can persist while compute dies.

---

## 4. The Cloudflare Tunnel trick

The single highest-leverage free-tier move:

```
Internet → yourdomain.com → Cloudflare (DNS + WAF + CDN + TLS)
                                 ↓ cloudflared tunnel (outbound only)
                          Mac k3d cluster ("on-prem datacenter")
```

Your laptop becomes an internet-reachable site with a real domain, real certificates, a
real WAF, and real edge caching — **with no inbound ports open and no public IP**. That
makes the `onprem` tier architecturally legitimate rather than a toy, and it gives you a
genuine hybrid-cloud topology to design around.

---

## 5. Cost controls (built in Phase 1, non-negotiable)

| Control | Implementation |
|---|---|
| **Nightly destroy** | Scheduled GitHub Action runs `terraform destroy` on `aws-lab` at 02:00 daily. Bypass requires an explicit label on an open PR. |
| **TTL tags** | Every ephemeral resource tagged `ttl=<hours>`; a reaper Action deletes expired ones. |
| **Budget alarms** | AWS Budgets, Azure Cost Alerts, GCP Budget — all wired to the same notification channel as prod alerts. |
| **Infracost in PRs** | Cost delta commented on every IaC PR. A `>$20/mo` increase requires justification in the PR body. |
| **Spot + arm64 everywhere** | Azure Spot node pools, AWS Graviton. Forces interruption handling — a real skill. |
| **Scale-to-zero** | `dev` and `stage` node pools scale to zero outside working hours via KEDA cron scalers. |
| **Kill switch** | `make nuke` destroys every cloud resource across all three providers. Tested monthly. |

**Phase 1 exit gate:** cost controls exist *before* the first expensive resource does.

---

## 6. Local resource budget (16 GB arm64 Mac)

The hard constraint. Profiles exist so you can choose what runs:

| Profile | Contains | Approx RAM |
|---|---|---|
| `core` | 5 Phase-0 services + Postgres + Redis + Kafka (KRaft, 1 broker) | ~6 GB |
| `obs` | `core` + OTel Collector + Prometheus + Grafana + Loki + Tempo | ~10 GB |
| `full` | `obs` + Argo CD + Kyverno + cert-manager | ~13 GB — tight |

Rules for arm64:
- Use **OrbStack**, not Docker Desktop (roughly half the overhead on Apple Silicon).
- Verify arm64 images exist before adopting a tool; some Confluent/Flink/older vendor
  images are amd64-only and emulation will destroy your RAM budget.
- Build multi-arch (`linux/amd64,linux/arm64`) from day one — cloud nodes may differ.
- **Multi-cluster local is not viable.** Multi-cluster work happens on Azure.

---

## 7. Account setup order

Do **not** create everything now. Credits are clocks.

| When | Action |
|---|---|
| **Now** | Cloudflare account (free) · GitHub public repo · Azure via MSDN · Grafana Cloud free |
| **Phase 0** | Nothing purchased. Everything is local; TLS is handled by a **local CA (`mkcert`)**. |
| **Phase 1** | **Domain purchase** + NS delegation to Cloudflare · AWS account (12-month free-tier clock starts — acceptable, it's long) |
| **Phase 4** | **GCP account** (90-day $300 clock starts — only when you're ready to use it) |

> Buy the domain a few days *before* starting Phase 1, not on the day. Registrar identity
> verification and nameserver delegation are not instant, and neither belongs on the
> critical path.

---

## 8. Security posture for free accounts

Free-tier accounts get compromised and generate enormous bills. Before the first resource:

- [ ] MFA on the root/owner account of every provider; root credentials never used again
- [ ] Billing alerts at 25% / 50% / 80% / 100% of budget
- [ ] No long-lived access keys anywhere — CI authenticates via OIDC federation only
- [ ] `gitleaks` in pre-commit **and** CI before the first commit
- [ ] Public repo: verify nothing sensitive is committed (state files, tfvars, kubeconfigs)
      — `.gitignore` and `.gitattributes` reviewed before `git push`
- [ ] Remote Terraform state encrypted, in a private bucket, **never** in the public repo

---

## 9. Domain & DNS plan

**Lab domain: `kshitizmichael.xyz`** — registered at a third-party registrar, nameservers
delegated to Cloudflare. DNS, WAF, CDN, and Tunnel remain free regardless of registrar.

**Purchased at the start of Phase 1, not before.** Phase 0 is entirely local and uses a
local CA (`mkcert`); a public domain buys nothing there and only starts a renewal clock.
Phase 1 requires it for ACME DNS-01 challenges, publicly-trusted certificates, and named
Cloudflare Tunnels.

### Why a separate lab domain

A domain used for learning DNS, TLS, and email **will** accumulate damage: malformed SPF
records, failed DMARC alignment, test mail sent to real inboxes, subdomains flagged by
reputation services. That damage is slow and sometimes permanent. Isolating it from the
domain attached to a real name and job applications is deliberate, not a shortcut.

`kshitizmichael.in` is purchased later, for personal use only, and never used for lab
experiments.

### Registrar hygiene (done at purchase, no paid add-ons)

- [ ] 2FA on the registrar account (authenticator app or hardware key, **not SMS**)
- [ ] 2FA on the account's recovery email — the real attack path
- [ ] Transfer lock enabled (`clientTransferProhibited`) — free, standard
- [ ] Auto-renew on, valid payment method, monitored contact address
- [ ] All upsells declined: "domain protection", SSL, hosting, email, SEO

> Every control in the paid "protection" bundle is available free. Expiry, not theft, is
> how domains are actually lost.

### Subdomain layout

```
kshitizmichael.xyz                      landing / project index
├── trackline.kshitizmichael.xyz        prod            (Azure AKS)
│   ├── stage.trackline....             stage           (Azure AKS)
│   ├── dev.trackline....               dev             (Azure AKS)
│   └── pr-<n>.trackline....            PR previews     (ephemeral namespace)
├── onprem.kshitizmichael.xyz           Mac k3d via Cloudflare Tunnel
├── aws.kshitizmichael.xyz              aws-lab         (ephemeral, nightly destroy)
├── gcp.kshitizmichael.xyz              gcp-dr          (Phase 5)
├── argocd.kshitizmichael.xyz           Argo CD
├── grafana.kshitizmichael.xyz          Grafana
├── vault.kshitizmichael.xyz            Vault           (Phase 4)
└── backstage.kshitizmichael.xyz        IDP             (Phase 7)
```

**Nested naming is chosen deliberately.** A wildcard certificate matches exactly one
label, so `*.kshitizmichael.xyz` does **not** cover
`pr-142.trackline.kshitizmichael.xyz`. A second wildcard
(`*.trackline.kshitizmichael.xyz`) is required, issued via its own DNS-01 challenge.
This limitation is accepted on purpose — encountering it during Phase 2 design is
preferable to encountering it during a Phase 2 outage.

### Consequences of `.xyz` to be handled

| Issue | Mitigation | Phase |
|---|---|---|
| Not on the HSTS preload list (unlike `.dev`/`.app`), so plaintext HTTP can silently "work" | Submit the apex to the HSTS preload list manually; enforce HSTS headers at the edge | 4 |
| Poor sending reputation — `.xyz` is filtered aggressively | `notifier` sends to a local sink (Mailpit) in every environment; real delivery is a single scoped exercise using a transactional provider's sandbox, never bulk | 3 |
| Reputation services may flag lab subdomains | Accepted. This is precisely why the personal domain is kept separate. | — |

---

## 10. Cost model

### 10.1 Total cash cost, first full pass

| Item | Cost |
|---|---|
| Domain (`.xyz`, first year promo) | ~$2 |
| Azure prod/dev/stage | **$0 cash** — absorbed by the $50/mo MSDN credit |
| AWS `aws-lab` (ephemeral, Phases 5–6 only) | ~$20–40 total |
| GCP `gcp-dr` (Phase 5 window) | $0 — $300 credit |
| Cloudflare, GitHub, Grafana Cloud | $0 |
| Local (Mac) | $0 |
| **Total, Phases 0–7** | **~$25–45** |

### 10.2 Recurring after the first pass

| Item | Cost |
|---|---|
| Domain renewal | ~$12–18/yr |
| Azure | $0 while the MSDN credit lasts |
| AWS | $0 if `aws-lab` stays destroyed |
| **Steady state** | **~$1–2/month** |

### 10.3 Fitting Azure prod inside $50/month

The MSDN credit is tight, not generous. These levers are **required**, not optional:

| Lever | Saves / month |
|---|---|
| **Cloudflare Tunnel instead of an Azure Load Balancer + public IP** | ~$22 |
| **Spot node pools only** (accept eviction; handling it is the lesson) | ~70–90% of compute |
| **Scale `dev`/`stage` node pools to zero outside working hours** | ~60% of non-prod |
| **Kafka self-hosted via Strimzi — never managed** | $100–540 |
| **Redis in-cluster — never Azure Cache** | ~$16 |
| **Managed Postgres in `prod` only**; CloudNativePG in `dev`/`stage` | ~$34 |
| Standard SSD / ephemeral OS disks, not Premium | ~$8 |

Resulting `prod` footprint: 2× Spot D2as_v5 + managed Postgres (Burstable B1ms) +
disks ≈ **$35–45/month**. It fits, with little room. Treat the credit as a hard SLO.

### 10.4 Cost traps that produce surprise bills

Each of these has ended someone's free-tier experiment. Verify before provisioning.

| Trap | Real cost | Avoidance |
|---|---|---|
| **Managed Kafka** (MSK, MSK Serverless, Event Hubs Std) | $100–540/mo | Strimzi in-cluster, always |
| **EKS control plane left running** | $73/mo | Nightly destroy, enforced by scheduled workflow |
| **NAT Gateway left running** | ~$32/mo + $0.045/GB | Destroyed with the lab; consider public subnets for lab-only workloads |
| **Cloud load balancers** | ~$18–25/mo each | Cloudflare Tunnel; one shared ingress, never one LB per service |
| **Cross-AZ traffic** (Kafka replication is the classic) | $0.01/GB each way | Single-AZ for lab clusters; accept the reduced availability deliberately |
| **Egress during load tests** | $0.087–0.12/GB | Mega-scale `device-sim` runs stay **inside** the VPC or **local**. Never drive 1M devices at cloud scale from outside. |
| **GPU node pools left on** | $500–3000/mo | `eta-ml` uses CPU inference; GPU only in a timed, alarmed window |
| **Idle managed services** (Redis, search, registries) | $15–50/mo each | In-cluster equivalents; managed only where the *managed* part is the lesson |
| **Log/metric ingestion overage** | $10–100s/mo | Cardinality limits, sampling, retention caps — set *before* first ingest |

### 10.5 The largest financial risk

Not the planned spend — the **forgotten resource**. A NAT Gateway left running over a
holiday costs more than this entire project. This is why every control in §5 is a Phase 1
exit criterion, and why `make nuke` is tested monthly rather than written and trusted.
