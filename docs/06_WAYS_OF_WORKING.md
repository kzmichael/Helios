# Ways of Working

How work is broken down, ordered, tracked, and accepted.

---

## 1. Tooling decision

**Backlog source of truth:** YAML files in `backlog/`, version-controlled in this repo.
**Board:** **Azure DevOps Boards**, populated from those files via `az boards` (Azure CLI).
**Code, CI, and registry:** GitHub — linked to Boards via the Azure Boards GitHub App.

### Why Azure DevOps Boards

The requirement was dependency tracking and ordering. GitHub Projects cannot express
either: it has no work-item hierarchy and no typed links.

| Capability | GitHub Projects | **Azure DevOps Boards** |
|---|---|---|
| Hierarchy | Flat issues, task lists | **Epic → Feature → User Story → Task**, native |
| Dependencies | Text references | **Predecessor/Successor, Parent/Child, Related** |
| Querying | Filters | **WIQL**, saved queries, dashboards |
| Iterations | A field | Native sprints, burndown, capacity |
| Skill transfer | Limited | Matches enterprise practice |

ADO is available at no cost through the MSDN subscription, and Phase 2 already requires
an Azure DevOps pipeline — so the platform is in scope regardless.

### Why not self-hosted Jira/Plane/OpenProject

| Option | Verdict |
|---|---|
| Jira Cloud (free tier) | Another vendor, no advantage over ADO here |
| Self-hosted Plane / OpenProject / Redmine | Consumes 2–4 GB of a 16 GB RAM budget, becomes a production system *you* must keep alive, and its outages block your actual work |
| Markdown checklists only | No dependencies, no queryable state, no board — the original problem |
| **Backlog-as-code → ADO Boards** | **Chosen.** Definition is diffable and reviewable in PRs; execution state lives in a real enterprise board. |

### Split of responsibilities between Git and ADO

To make the sync one-way and conflict-free:

| Owned by `backlog/*.yaml` (Git) | Owned by ADO Boards |
|---|---|
| Title, description, persona, value | State / board column |
| Acceptance criteria, Definition of Done | Effort, remaining work |
| Dependencies, epic, phase, tags | Iteration assignment |
| Study references | Comments, discussion, attachments |

The sync **creates and updates definition fields only**. It never reads state back and
never overwrites execution state. Nothing can conflict.

### Work item mapping

Process template: **Agile** (Basic lacks a Feature level; Scrum's states add friction for
solo work).

| HELIOS | ADO work item type |
|---|---|
| Phase 0–8 | **Epic** |
| Epic (`E0.3`) | **Feature** |
| Story (`HEL-0.3.1`) | **User Story** |
| Spike (`SPK-0.1`) | **User Story**, tagged `spike` |
| Sub-steps | **Task** (optional, not tracked in YAML) |

`depends_on` becomes a **Predecessor/Successor** link — the mechanism that makes ordering
real rather than advisory.

### GitHub ↔ ADO linkage

Code, Actions, and GHCR stay on GitHub (free unlimited minutes on a public repo). The
**Azure Boards GitHub App** links them: putting `AB#<id>` in a commit message or PR title
attaches it to the work item, and `Fixes AB#<id>` transitions it on merge.

### The sync script is your work

Writing `scripts/backlog-sync.sh` (YAML → `az boards work-item create/update` → link
parents and predecessors) is automation, therefore operator work. It is tracked as story
`HEL-0.6.1`. Until it exists, work the backlog file directly — do not let tooling block
Phase 0.

---

## 2. Work breakdown hierarchy

| Level | Meaning | ID format |
|---|---|---|
| **Phase** | A stage of the roadmap; the "initiative" | Phase 0–8 |
| **Epic** | A coherent outcome within a phase | `E0.3` |
| **Story** | One deliverable slice with acceptance criteria | `HEL-0.3.2` |
| **Spike** | Time-boxed *learning*, with a self-check as acceptance | `SPK-0.1` |
| **Task** | Optional sub-step inside a story; not separately tracked | — |

### Spikes are first-class here

In a normal enterprise backlog, "study X" is not a ticket. In this project it is the
majority of the value, so **learning is tracked as work**. Every spike references
`05_STUDY_GUIDE.md` and takes that phase's self-check questions as its acceptance criteria.

A spike is Done when you can answer the questions **from memory, out loud**. Not when you
have finished reading.

---

## 3. Story format

```yaml
- id: HEL-0.3.2
  title: Short imperative statement
  type: story | spike | chore
  epic: E0.3
  persona: who benefits
  value: why this matters (the "so that")
  depends_on: [HEL-0.3.1]
  study: [05_STUDY_GUIDE.md#containers]
  estimate: S | M | L
  skills: [docker, bash]
  description: >
    Context and constraints. What is in scope, what is not.
  acceptance_criteria:
    - Given/When/Then, or a verifiable statement
  done_when:
    - Objectively checkable outcomes
```

**Acceptance criteria must be testable by someone else.** "Docker works" is not a
criterion. "`docker run --rm <image> id -u` outputs a non-zero UID" is.

---

## 4. Personas

The gap between "business" and "tech" is real even in a solo project — these are who each
story is *for*. Writing them forces the "why".

| Persona | Cares about |
|---|---|
| **Fleet Manager** (Trackline customer) | Vehicles visible on the map, alerts arriving, invoices correct |
| **Device** | Telemetry accepted, backpressure handled, never silently dropped |
| **App Developer** | Fast local loop, easy deploys, clear ownership, working preview envs |
| **On-call Engineer** | Actionable alerts, working runbooks, fast diagnosis, no noise |
| **Security Engineer** | No static credentials, verified artifacts, enforced policy, audit trail |
| **Platform Owner** (you) | Reproducible infra, low toil, predictable cost |
| **Budget Holder** | Spend inside the credit, no surprise bills, cost attributable |

---

## 5. Definition of Ready

A story may not be started unless:

- [ ] Acceptance criteria are written and testable
- [ ] Dependencies are listed and satisfied
- [ ] Required study (`study:` refs) is complete or is the story itself
- [ ] It fits in one working session block (if not, split it)
- [ ] The persona and value are stated — if you can't name who benefits, question the story

## 6. Definition of Done

Universal, applies to every story:

- [ ] Acceptance criteria all pass
- [ ] Code/config committed, no manual console changes
- [ ] Works from scratch on a clean environment (destroy and rebuild, don't assume)
- [ ] Documented: README, runbook, or inline — wherever a stranger would look
- [ ] Secrets scanned; nothing sensitive committed
- [ ] Cost impact understood and acceptable
- [ ] You can explain *why* it works, not only that it does

---

## 7. Board

Columns, with a hard **WIP limit of 2** in progress:

```
New → Ready → Active → Blocked → In Review → Closed
```

ADO configuration:

| Setting | Value |
|---|---|
| Process | Agile |
| Area paths | `HELIOS/App`, `/Platform`, `/Infra`, `/Observability`, `/Security`, `/FinOps` |
| Iterations | Weekly sprints, `Sprint 1`, `Sprint 2`, … |
| Tags | `phase-0`, `spike`, skill tags (`docker`, `k8s`, `terraform`, …) |
| Story Points | S = 1, M = 3, L = 8 |
| Board columns | Split `Active` into Doing/Done to expose handoffs |

**Anything larger than L must be split.** An "L" that grows is a planning failure, and
noticing that is part of the exercise.

---

## 8. Cadence

Solo, so ceremonies are compressed but not skipped:

| Ritual | When | Output |
|---|---|---|
| **Iteration planning** | Start of each week | Pull stories into Ready; commit to a weekly goal |
| **Daily check** | Each session start | What's in progress, what's blocked, is WIP ≤ 2 |
| **Grooming** | Mid-phase | PO (me) writes the *next* phase's backlog — never more than one phase ahead |
| **Review** | Phase exit | Walk the exit criteria; demo to yourself; record what's incomplete |
| **Retro** | Phase exit | `ops/postmortems/phase-N-retro.md` — what broke, what surprised you, what changes |

**Only one phase is groomed ahead.** Writing all eight phases of stories now would be
waste; the later ones would be wrong by the time you reached them.

---

## 9. Dependency tracking

Dependencies live in the `depends_on:` field and are mirrored into ADO as
**Predecessor/Successor** links. Three kinds matter here:

| Kind | Example |
|---|---|
| **Hard technical** | Cannot deploy to AKS before the cluster exists |
| **Knowledge** | Cannot write a NetworkPolicy before understanding CNI |
| **Cost/sequencing** | Cannot create the GCP account before Phase 4 (credit clock) |

The knowledge dependencies are the ones people skip and then fail on. They are enforced
here by spikes blocking their dependent stories.

---

## 10. Backlog files

| File | Status |
|---|---|
| `backlog/phase-0.yaml` | Groomed — ready to work |
| `backlog/phase-1.yaml` | Written mid-Phase 0 |
| later phases | Not yet written, deliberately |
