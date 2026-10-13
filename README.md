# Project HELIOS

A production-grade, multi-cloud platform built to exercise every discipline of DevOps,
SRE, Platform Engineering, and SecOps — hands-on, end to end.

The workload is **Trackline**, a multi-tenant fleet telemetry SaaS deliberately designed
to generate real operational pressure: streaming ingest, real-time delivery, batch
billing, ML inference, and multi-tenant isolation.

---

> ## 👉 New here, or feeling lost? Read **[START_HERE.md](START_HERE.md)** first.
>
> It explains the entire project in plain language — every technology, what it does, and
> why it's here — then gives you a step-by-step action plan with the reasoning behind
> each step. The documents below are references; that one is the map.

---

## Start here

| Read | For |
|---|---|
| **[START_HERE.md](START_HERE.md)** | **Plain-language explanation of everything + the step-by-step action plan** |
| [docs/00_CHARTER.md](docs/00_CHARTER.md) | Purpose, scope, principles, success criteria |
| [docs/01_APPLICATION.md](docs/01_APPLICATION.md) | Trackline architecture, services, data layer, SLO candidates |
| [docs/02_TECH_MATRIX.md](docs/02_TECH_MATRIX.md) | Every technology, Phase 1 vs Phase 2, target depth |
| [docs/03_ROADMAP.md](docs/03_ROADMAP.md) | **Step-by-step execution guide**, verification evidence, phase gates, and weighted first-lap progress tracker |
| [docs/04_ENVIRONMENTS.md](docs/04_ENVIRONMENTS.md) | Cloud accounts, budget strategy, cost model, cost controls |
| [docs/05_STUDY_GUIDE.md](docs/05_STUDY_GUIDE.md) | **What to study, per phase**, with self-check questions |
| [docs/06_WAYS_OF_WORKING.md](docs/06_WAYS_OF_WORKING.md) | How work is broken down, tracked, and accepted |
| [docs/07_REPOSITORY_BLUEPRINT.md](docs/07_REPOSITORY_BLUEPRINT.md) | **Every file/folder needed end to end**, phase-tagged |
| [backlog/phase-0.yaml](backlog/phase-0.yaml) | **The groomed Phase 0 backlog** — start here to work |
| [docs/adr/](docs/adr/) | Architecture Decision Records |

---

## Planned repository layout

```
helios/
├── apps/                      # Application source (provided)
│   ├── _proto/                # Shared protobuf + event schemas
│   ├── edge-gateway/          # Go   — public API, authZ, rate limiting
│   ├── ingest/                # Go   — telemetry sink (gRPC + MQTT)
│   ├── fleet-api/             # TS   — tenants, fleets, vehicles
│   ├── notifier/              # Rust — webhooks, email, SMS
│   ├── web/                   # Next — dashboard + live map
│   └── device-sim/            # Go   — synthetic fleet load generator
│
├── infra/                     # Infrastructure as Code (built by hand)
│   ├── terraform/{modules,live/{dev,stage,prod,aws-lab,gcp-dr}}
│   ├── bicep/                 # Azure prod tier, second implementation
│   ├── pulumi/                # Cloudflare edge layer
│   ├── crossplane/            # Self-service compositions (Phase 7)
│   └── ansible/{roles,inventories,molecule}
│
├── deploy/
│   ├── kustomize/{base,overlays/{dev,stage,prod,preview}}
│   ├── helm/                  # One service, for comparison
│   ├── argocd/                # App-of-apps, ApplicationSets
│   └── policies/              # Kyverno / Gatekeeper admission policies
│
├── observability/
│   ├── otel/                  # Collector configs, pipelines
│   ├── dashboards/            # Grafana dashboards as code
│   ├── alerts/                # Prometheus rules, burn-rate alerts
│   └── slo/                   # SLO definitions (Sloth / Pyrra)
│
├── security/
│   ├── policies/ trivy/ sbom/ threat-models/
│
├── platform/                  # Backstage, scaffolder templates, scorecards
│
├── ops/
│   ├── runbooks/ postmortems/ gamedays/ dr/
│
├── tests/
│   ├── load/                  # k6 suites
│   ├── chaos/                 # Chaos Mesh / Litmus experiments
│   └── contract/              # Pact
│
├── docs/{adr,diagrams}
├── .github/workflows/
└── Taskfile.yml
```

---

## Division of labour

| Provided | Built by hand |
|---|---|
| Application source, schemas, API contracts, seed data | All IaC, pipelines, manifests, policies, dashboards, runbooks |
| Architecture options and tradeoffs | The decision and the implementation |
| Study targets and proof-of-competence tasks | The studying and the building |

Application development is handed over. Everything an SRE, DevOps, Platform, or SecOps
engineer would own is built by hand.

---

## Status

| Phase | Name | State |
|---|---|---|
| 0 | Foundations & Application | Not started |
| 1 | First Production | — |
| 2 | GitOps & Progressive Delivery | — |
| 3 | Observability, SLOs & On-Call | — |
| 4 | Security & Supply Chain | — |
| 5 | Multi-Cloud, Multi-IaC & Config Mgmt | — |
| 6 | Resilience, Scale & FinOps | — |
| 7 | Platform Engineering | — |
| 8+ | Advanced Pass | — |
