# Project HELIOS — Charter

> A production-grade, multi-cloud platform built to exercise every discipline of
> DevOps, SRE, Platform Engineering, and SecOps — end to end, hands-on.

---

## 1. Purpose

Build and operate a real multi-tenant SaaS product (**Trackline**, see `01_APPLICATION.md`)
across multiple clouds and multiple toolchains, so that every technology learned has a
*reason to exist* and a *failure mode you have personally debugged*.

**Non-goal:** collecting tools. If a tool is added, it must solve a problem the platform
actually has. Every addition requires an ADR.

---

## 2. Roles being simulated

| Role | What HELIOS must prove |
|---|---|
| **DevOps Engineer** | CI/CD, IaC, config management, automation of toil |
| **SRE** | SLIs/SLOs/error budgets, on-call, incident response, chaos, capacity, DR |
| **Platform Engineer** | Golden paths, self-service, IDP, developer experience, paved roads |
| **SecOps / DevSecOps** | Supply chain, secrets, policy-as-code, runtime security, SIEM, compliance |
| **Cloud Architect** | Multi-cloud placement, cost, resilience, network design, tradeoff ADRs |

---

## 3. Division of labour

| Provided to the operator (Kshitiz) | Built by the operator |
|---|---|
| Application source code, schemas, API contracts, seed data | All IaC, pipelines, manifests, policies |
| Architecture options and tradeoffs | The decision + the implementation |
| Study targets per technology + proof-of-competence tasks | The studying and the building |
| Review, critique, failure-injection ideas | Debugging every breakage personally |

**Rule:** application development is handed over. Everything an SRE/DevOps/Platform/SecOps
engineer would own is built by hand. No copy-paste infra.

---

## 4. Operating principles

1. **Breadth first, depth second.** Phase 1 of a technology reaches L2 (working). The
   advanced pass comes in a later phase. Do not sink weeks into one tool early.
2. **Everything in Git.** No clicking in consoles except to *look*. If it exists, it was
   declared.
3. **Every phase ends with three artifacts:** an ADR, a retro/postmortem of what broke,
   and an updated runbook.
4. **Nothing runs that isn't observed.** No service ships without metrics, logs, traces,
   a dashboard, and an alert.
5. **Cost is a first-class SLI.** Ephemeral-by-default. Nightly destroy. Infracost in PRs.
6. **Break it on purpose.** Every phase includes at least one deliberate failure injection.

---

## 5. Definition of competence (depth scale)

| Level | Means |
|---|---|
| **L1 — Aware** | Can explain what/why/when; can read someone's config; can choose between alternatives |
| **L2 — Working** | Can build and operate it from docs; can debug the common 10 failures |
| **L3 — Fluent** | Can design systems with it; can tune it; knows its failure modes and limits |
| **L4 — Deep** | Knows internals; can extend it (plugin/operator/provider); can debug the tool itself |

**Phase 1 target:** L2 across the board, L3 on the *primary* tool of each category.

---

## 6. Commitment

- **Time budget:** 20+ hrs/week
- **Cash budget:** ~$0–15/mo out of pocket + $50/mo MSDN credit (see `04_ENVIRONMENTS.md`)
- **Hardware:** Apple Silicon Mac, 16 GB RAM, arm64

---

## 7. Success criteria

HELIOS is "done" with its first full lap when all of the following are true:

- [ ] A stranger can read `docs/` and understand the whole system without asking questions
- [ ] A new microservice can be scaffolded, built, deployed to prod, and observed via a
      single PR against a golden-path template
- [ ] Prod has published SLOs with burn-rate alerts that have fired for real at least once
- [ ] A full region failover has been executed and timed against a stated RTO/RPO
- [ ] A database restore has been performed from backup and verified — not just configured
- [ ] Unsigned images are rejected at admission, and this has been demonstrated
- [ ] There are zero long-lived cloud credentials anywhere in the system
- [ ] Cost per tenant is visible on a dashboard
- [ ] At least 15 ADRs and 5 postmortems exist

---

## 8. Document index

| Doc | Contents |
|---|---|
| `00_CHARTER.md` | This file — purpose, scope, principles, success criteria |
| `01_APPLICATION.md` | Trackline: services, data layer, contracts, SLO candidates |
| `02_TECH_MATRIX.md` | Every technology, Phase 1 vs Phase 2, target depth |
| `03_ROADMAP.md` | **Step-by-step execution guide**, verification evidence, phase gates, and weighted first-lap progress tracker |
| `04_ENVIRONMENTS.md` | Cloud/budget/account strategy, cost model, cost controls |
| `05_STUDY_GUIDE.md` | **Specific topics to study per phase**, with self-check questions |
| `06_WAYS_OF_WORKING.md` | Work breakdown, story format, Definition of Ready/Done, board, cadence |
| `07_REPOSITORY_BLUEPRINT.md` | **Every file and folder needed end to end**, phase-tagged, with what it's for |
| `adr/` | Architecture Decision Records |
| `../backlog/` | Groomed backlog as code — epics, stories, spikes, dependencies |
| `ops/` | Runbooks, postmortems, game day plans (created as phases complete) |
