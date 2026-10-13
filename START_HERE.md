# START HERE — Project HELIOS, Explained Properly

> If you read nothing else in this repository, read this file.
>
> **Part 1** explains what this project is, what every piece of technology does, and why
> it exists — in plain language, assuming you know almost nothing.
>
> **Part 2** is the action key: exactly what to do, in what order, and why that order.
>
> There is no word limit here on purpose. Skim it once to get the shape, then come back
> and read sections properly as you reach them.

---
---

# PART 1 — WHAT THIS PROJECT IS

---

## 1.1 The one-sentence version

You are going to build a fake-but-real software product, then build all the
infrastructure, automation, monitoring, and security around it that a real company would
need — so that you learn DevOps, SRE, Platform Engineering, and Security Operations by
doing the actual job rather than by watching tutorials.

---

## 1.2 Why this instead of courses

Courses teach you the tools. Nobody is paid to know tools. People are paid to keep systems
alive, and that skill only forms in three situations:

1. You built something end to end and understood every layer.
2. It broke and you fixed it.
3. You had to explain a decision to someone who disagreed.

A tutorial gives you none of those. This project is designed to give you all three,
deliberately and repeatedly.

The key insight behind the whole design: **infrastructure is only interesting when the
thing running on it is interesting.** If you deploy a "Hello World" app, then autoscaling
has nothing to scale, monitoring has nothing to watch, and a "chaos experiment" is just
you deleting a pod that nobody needed. So the first thing we did was invent a product
worth running.

---

## 1.3 The product: Trackline

Imagine a company that sells GPS tracking to logistics businesses.

A customer — say a trucking company — buys the service. They register their fleet of
vehicles. Each vehicle has a small GPS device bolted into it. That device sends its
position and some sensor readings (speed, fuel, engine temperature, ignition on/off) up to
Trackline, once every second, forever.

Trackline then:

- shows every vehicle moving on a **live map**
- detects when a truck **enters or leaves a defined area** ("geofence") — like a depot or a restricted zone
- detects **speeding** and **excessive idling**
- **raises alerts** and sends them out by email, SMS, or webhook
- **predicts arrival times** using a machine-learning model
- **bills the customer monthly** based on how many devices they used and how much data they sent
- lets them **query historical routes** for any vehicle

That's it. It's a believable business.

### Why this specific product was chosen

Because it is simultaneously many *different kinds* of system, and each kind creates a
different operational problem you need to learn to solve:

| Trackline is also a... | Which forces you to learn |
|---|---|
| **Request/response API** | Latency, load balancers, caching, connection pools |
| **High-volume data pipeline** | Queues, backpressure, throughput, partitioning |
| **Real-time system** | Freshness guarantees, WebSockets, push |
| **Batch system** | Scheduled jobs, correctness, retries, "money bugs" |
| **Machine-learning service** | Heavy compute, special hardware, model artifacts |
| **Multi-customer (multi-tenant) system** | Isolation, quotas, noisy neighbours, per-customer cost |
| **Stateful system** | Databases, backups, restores, failover, migrations |

A to-do app is only the first row. That's why it teaches you almost nothing.

### The three customers, and why they're unequal

The seed data creates three customers on purpose:

| Customer | Plan | Vehicles | Why they exist |
|---|---|---|---|
| Northwind Logistics | Free | 5 | The small, cheap customer you might have to sacrifice under load |
| Meridian Freight | Pro | 25 | The normal case |
| Atlas Continental | Enterprise | 120 | The big customer generating most of the load and most of the revenue |

They are **deliberately different sizes**. If all three were identical, you could never
demonstrate:
- **noisy neighbour** problems (one customer slowing down everyone else)
- **fair sharing** (making sure the big customer doesn't starve the small one)
- **cost attribution** (working out what each customer actually costs you to serve)
- **load shedding** (deciding who to drop first when you can't serve everyone)

Those four things are core SRE work. Equal customers make them invisible.

---

## 1.4 The nine services, in plain words

Trackline is not one program. It's several small programs that talk to each other. This is
called a **microservice** architecture. Each one is written in a different language on
purpose — because in a real job, you don't get to choose, and you must be able to
containerise, deploy, and debug code you didn't write.

### Built first (Phase 0) — the core five, plus a load generator

| Service | Language | What it actually does | Why you care |
|---|---|---|---|
| **edge-gateway** | Go | The front door. Every request from the internet hits this first. It checks who you are, whether you're allowed, and whether you're going too fast. Then it forwards you inward. | This is where TLS certificates, firewalls, rate limiting, and authentication all land. |
| **ingest** | Go | Receives the firehose of GPS data from devices and writes it into a queue. Nothing else. | This is the part that falls over under load. Autoscaling, backpressure, and load shedding all live here. |
| **fleet-api** | TypeScript | The boring-but-essential part. Create a fleet, add a vehicle, list vehicles, define a geofence. Talks to the database. | This is where database problems live: slow queries, connection exhaustion, migrations. |
| **notifier** | Rust | Takes an alert and actually delivers it — sends the email, calls the webhook, sends the SMS. Retries when it fails. | This teaches retries, idempotency, dead-letter queues, and depending on third parties who go down. |
| **web** | Next.js / React | The dashboard the customer looks at. The live map. | CDN, browser caching, WebSockets, real-user monitoring. |
| **device-sim** | Go | Fake GPS devices. You tell it "pretend to be 10,000 trucks driving around" and it does. | **This is your most important tool.** It's how you create load, trigger failures, and test everything. |

### Built later (Phase 2)

| Service | Language | What it does | Why later |
|---|---|---|---|
| **stream-proc** | Java (Flink) | Watches the stream of GPS points and works out "this truck just entered the depot" or "this truck has been speeding for 30 seconds" | Stateful stream processing is genuinely hard. Doing it in Phase 0 would stall you. |
| **eta-ml** | Python | Predicts arrival times and flags anomalies | Needs special compute. Adds cost and complexity. |
| **billing-batch** | Java (Spring) | Runs every night, adds up usage, produces invoices | Correctness matters more than speed here — a different discipline. |

---

## 1.5 The supporting cast — the data stores

These aren't services you write. They're off-the-shelf systems that store things.

| Thing | What it is, plainly | What Trackline uses it for |
|---|---|---|
| **PostgreSQL** | A traditional database. Tables, rows, columns. Strong guarantees. | The permanent record: customers, vehicles, invoices, alerts. |
| **Redis** | An extremely fast store that keeps everything in memory. Loses data if it restarts (by default). | Caching, counting requests for rate limits, last-known vehicle positions. |
| **Kafka** | A durable queue. Programs write messages onto it; other programs read them at their own speed. Messages stay for a set time even after being read. | The backbone. GPS data goes in one end and several different services read it independently. |
| **Object storage** (S3 / R2) | Somewhere to put files, cheaply, forever. | Old GPS data, backups, ML models. |

### Why Kafka instead of just calling the next service directly?

This is worth understanding because it's the central design idea.

If `ingest` called `fleet-api` directly, then when `fleet-api` is slow or down, `ingest`
is also stuck, and devices start failing. One slow component takes down the whole chain.

With Kafka in the middle, `ingest` just drops the message on the queue and moves on. If a
downstream service is slow, the queue gets longer — which is a problem, but a *visible,
survivable* problem instead of an outage. This is called **decoupling**, and "how long is
the queue" (**consumer lag**) becomes one of your most important health signals.

---

## 1.6 The infrastructure: what actually runs all this

Now the part you're learning. Here is the whole stack from the bottom up, in plain terms.

### Layer 1: Machines

Somewhere, there are computers. In the cloud they're called **virtual machines** or
**nodes**. They run Linux.

**Why you must know Linux well:** every single problem above this layer eventually turns
into a Linux problem. "The container is being killed" is really "the Linux kernel's
out-of-memory killer chose this process." "The service is slow" is really "we ran out of
file descriptors." If Linux is a black box to you, every layer above it is magic, and you
can't debug magic.

### Layer 2: Containers

A **container** is your application packaged with everything it needs to run — libraries,
config, the lot — so it behaves identically on your laptop and in the cloud.

An **image** is the frozen recipe. A **container** is a running copy of that image. You
build an image once and run it in many places. **Docker** is the tool that builds and runs
them.

**Why this matters:** before containers, "works on my machine" was a genuine, constant,
expensive problem. Containers make the environment part of the artifact.

### Layer 3: Kubernetes

If you have 40 containers across 6 machines, something has to decide which container runs
where, restart them when they crash, replace them when a machine dies, and give them
network addresses. That something is **Kubernetes** (often written "k8s").

Think of it as an operating system for a whole datacenter. You don't tell it "run this
container on that machine." You tell it "I want 5 copies of this running at all times,"
and it continuously makes that true. That continuous-making-it-true idea is called
**reconciliation**, and it's the single most important concept in Kubernetes.

Vocabulary you'll meet immediately:

| Term | Meaning |
|---|---|
| **Pod** | The smallest unit Kubernetes runs — usually one container |
| **Deployment** | "Keep N copies of this pod running" |
| **Service** | A stable internal address for a set of pods |
| **Ingress / Gateway** | How traffic from the internet gets in |
| **Namespace** | A folder for grouping and isolating things |
| **Manifest** | A YAML file describing what you want |
| **Node** | One machine in the cluster |

### Layer 4: Infrastructure as Code

You *could* click buttons in the AWS or Azure web console to create servers. Almost
everyone starts there. It's a trap, because:

- you can't review a click
- you can't undo a click reliably
- you can't recreate last Tuesday's setup
- you can't tell what changed or who changed it

So instead you write files that describe the infrastructure you want, and a tool makes
reality match. **Terraform** (or its open fork **OpenTofu**) is the main one. You write
"I want a Kubernetes cluster with 3 nodes and a database," run `terraform apply`, and it
builds it. Run it again, nothing changes. Delete the file and apply, it removes it.

This is called **declarative** — you declare the destination, not the route.

**Ansible** is a cousin that solves a different problem: configuring the inside of machines
that already exist (install this, set this config file, start this service). Terraform
creates the house; Ansible furnishes it.

### Layer 5: CI/CD — getting your code from laptop to production

**CI (Continuous Integration):** every time you push code, a robot automatically builds it,
runs the tests, scans it for vulnerabilities, and produces a container image. If anything
fails, the change doesn't proceed. **GitHub Actions** is our main one.

**CD (Continuous Delivery/Deployment):** the built image gets deployed to real
environments, automatically.

There are two styles, and the difference matters:

| Style | How it works | Problem |
|---|---|---|
| **Push** (old) | Your CI system has credentials to your cluster and pushes changes in | Your CI system is now a master key to production. If GitHub is compromised, so are you. |
| **Pull / GitOps** (modern) | A program *inside* the cluster watches Git and pulls changes in itself | No external system holds cluster credentials. It also notices and reverts manual changes. |

**Argo CD** is the program that does the pulling. It constantly compares "what Git says"
with "what's actually running" and fixes the difference. If someone manually changes
something in the cluster, Argo puts it back within seconds. That's called **drift
detection**.

### Layer 6: Observability — knowing what's happening

You cannot operate what you cannot see. There are five kinds of signal:

| Signal | What it is | Example | Tool |
|---|---|---|---|
| **Metrics** | Numbers over time, cheap to store | "requests per second = 412" | Prometheus |
| **Logs** | Text records of events | "failed to connect to database" | Loki |
| **Traces** | The full journey of one request across all services, with timings | "this request took 2.3s: 0.1s in gateway, 2.1s waiting on database" | Tempo |
| **Profiles** | Which lines of code are using the CPU | "40% of time is in JSON parsing" | Pyroscope |
| **Events** | Discrete things that happened | "deployment v1.4.2 started" | Kubernetes / Grafana |

**Grafana** draws all of it on dashboards.

**OpenTelemetry (OTel)** is the important one. It's a vendor-neutral standard for producing
these signals. You instrument your code **once** with OpenTelemetry, then send the data
anywhere — Prometheus, Grafana Cloud, Azure Monitor, Datadog, all at once if you like.

> This is why the plan says *"don't install every monitoring vendor's agent."* Installing
> five agents teaches you five installers. Instrumenting once with OTel and fanning out to
> five backends teaches you the actual skill, and it's what real companies do to avoid
> being locked to one vendor.

### Layer 7: SLOs — deciding how reliable is reliable enough

This is the heart of SRE, and it's the bit most people skip.

- **SLI** (Service Level *Indicator*): a measurement of something users care about.
  Example: "percentage of map requests that load in under 300ms."
- **SLO** (Service Level *Objective*): the target for that measurement.
  Example: "99.9% of map requests load in under 300ms, measured over 30 days."
- **Error budget**: the amount you're allowed to fail. If your SLO is 99.9%, your error
  budget is 0.1% — about 43 minutes a month.

The error budget is the clever part. It turns an argument into arithmetic:

- Budget remaining? Ship features fast, take risks.
- Budget spent? **Stop shipping features. Fix reliability.** Not as a punishment — as a
  pre-agreed rule.

It also stops the two classic failure modes: chasing 100% uptime (infinitely expensive,
and pointless because the user's WiFi is less reliable than that), and having no target at
all (so every outage is an argument about whether it mattered).

### Layer 8: Security

Split into a few concerns:

| Concern | The question | Tools |
|---|---|---|
| **Identity** | Who is this, and what may they do? | OIDC, JWT, workload identity |
| **Secrets** | Where do passwords and keys live, and how do they rotate? | Vault, External Secrets |
| **Supply chain** | Do I know where this container image came from, and has it been tampered with? | SBOM, cosign, SLSA |
| **Policy** | What is not allowed to run here, ever? | Kyverno, OPA |
| **Runtime** | Is something bad happening right now? | Falco, SIEM |

The most important single idea: **no long-lived credentials.** A password in a config file
lives forever and leaks eventually. Modern systems use short-lived, automatically-rotated
credentials that a machine proves it deserves (this is what "workload identity" and "OIDC
federation" mean). Phase 4 of this project exists mainly to make that real for you.

### Layer 9: Platform Engineering

Once all of the above exists, it's too complicated for ordinary developers to use. So a
platform team wraps it in something friendly: a catalogue of services, templates that
generate a new service with monitoring and deployment already wired up, and self-service
so a developer can get a database by opening a pull request instead of filing a ticket.

**Backstage** is the catalogue. **Crossplane** is the self-service. This is Phase 7, and
it's the highest-paid version of this job.

---

## 1.7 Following one GPS point all the way through

Concrete beats abstract. Here is what happens when one truck reports its position, and
which technology is responsible at each step.

```
 1. GPS device in a truck records: lat 12.97, lon 77.59, speed 64 km/h
        │
 2. Sends it over the internet, encrypted           ← TLS certificate
        │
 3. Arrives at Cloudflare                            ← DNS, WAF, DDoS protection
        │
 4. Enters the Kubernetes cluster                    ← Ingress / Gateway API
        │
 5. `ingest` checks the device's token               ← authentication
        │
 6. `ingest` checks the customer isn't over-rate     ← rate limiting (Redis)
        │
 7. `ingest` writes it to Kafka and immediately      ← the decoupling point
    replies "got it"
        │
        ├──▶ 8a. `position-updater` reads it and updates the
        │        "latest position" table + Redis         ← this is what the map reads
        │
        ├──▶ 8b. `telemetry-archiver` batches it into Postgres,
        │        later into object storage              ← history, cheap storage
        │
        └──▶ 8c. `stream-proc` notices the truck just crossed
                 a geofence boundary                    ← stateful stream processing
                     │
                 9.  publishes a "geofence entered" event to Kafka
                     │
                10.  `alert-engine` matches it to a customer's rule
                     and raises an alert
                     │
                11.  `notifier` sends the email/webhook  ← retries, idempotency, DLQ
                     │
                12.  The dashboard updates over WebSocket ← the live map moves
```

Meanwhile, invisibly, at every step:

- a **trace** is being stitched together so you can see the timing of all 12 steps as one picture
- **metrics** are counting requests, errors, and durations
- **logs** are recording anything unusual, tagged with the same trace ID
- the **error budget** for "map freshness under 5 seconds" is being consumed or preserved

**Step 12 minus step 1 is one of your SLOs.** Everything in the middle exists to keep that
number small. That's not an accident — the architecture was chosen to make that SLO
achievable.

---

## 1.8 Where it all runs

You are not using one cloud. Each has a job, chosen partly for learning and partly for
cost.

| Place | What runs there | Why there |
|---|---|---|
| **Your Mac** | Everything, for development. Also pretends to be an "on-premises datacenter." | Free. Fast feedback loop. |
| **Azure** (free $50/mo MSDN credit) | The permanent production system: dev, stage, prod | Its Kubernetes control plane is **free**; AWS charges $73/month for the same thing. Since SRE work needs an always-running production (30-day SLO windows, on-call, error budgets), Azure is the only affordable option for always-on. |
| **AWS** (free tier) | A lab that is **destroyed every night** | AWS is the market leader so you need real depth in it. Destroying it nightly forces every single thing into Terraform — you physically cannot get away with clicking buttons. |
| **GCP** ($300 free credit) | Disaster-recovery site + machine learning | Teaches cross-cloud failover and a second serverless model. Created late, because the credit is a 90-day clock. |
| **Cloudflare** (free) | DNS, firewall, CDN, and a tunnel to your Mac | Genuinely production-grade at zero cost. |

### The trick that makes the whole thing work on no money

**Cloudflare Tunnel.** Your Mac makes an *outbound* connection to Cloudflare. Cloudflare
then routes real internet traffic, on your real domain, with a real certificate, down that
tunnel to your laptop.

No public IP. No open firewall ports. No cloud bill.

This means your laptop is a legitimate part of the architecture — a real "on-premises
site" in a hybrid cloud design — not a toy. It's also why Ansible has something real to
manage.

---

## 1.9 What it costs

| Item | Cost |
|---|---|
| Domain name | ~$2 the first year |
| Azure (prod/dev/stage) | $0 — covered by the MSDN credit |
| AWS (ephemeral lab) | ~$20–40 total, across the whole project |
| GCP | $0 — free credit |
| Cloudflare, GitHub, Grafana Cloud | $0 |
| **Total for the entire project** | **~$25–45** |

The biggest financial risk is not planned spending — it's **forgetting to turn something
off**. One NAT Gateway left running over a holiday costs more than this entire project.
That's why building the cost controls is a Phase 1 *exit requirement*, before any
expensive resource exists.

---

## 1.10 Glossary

Keep this open. Almost every confusing sentence in the other documents is one of these
words.

### General

| Term | Plain meaning |
|---|---|
| **Artifact** | A built thing you deploy — usually a container image |
| **Registry** | Where images are stored (GitHub's is GHCR) |
| **Immutable** | Never modified after creation; to change it, build a new one |
| **Idempotent** | Safe to run twice. Running it again changes nothing new. |
| **Declarative** | You describe the end state; the tool works out the steps |
| **Reconciliation** | A loop that continuously makes reality match the description |
| **Drift** | Reality has diverged from the description |
| **Blast radius** | How much breaks when this breaks |
| **Toil** | Manual, repetitive work that should be automated. SRE exists to reduce it. |

### Operations

| Term | Plain meaning |
|---|---|
| **Runbook** | Written instructions for handling a specific alert or task |
| **Postmortem** | Written analysis after an incident — what happened, why, what changes |
| **Blameless** | The postmortem examines the system, not the person |
| **On-call** | Being the person the alert wakes up |
| **MTTR** | Mean Time To Recovery — how long outages last on average |
| **p99** | The value 99% of requests are faster than. p99 = 300ms means 1 in 100 users waited longer. |
| **RTO** | How long you're allowed to take to recover from a disaster |
| **RPO** | How much data you're allowed to lose in a disaster |

### Deployment

| Term | Plain meaning |
|---|---|
| **Canary** | Send 5% of traffic to the new version; if it looks fine, send more |
| **Blue/green** | Run both versions; flip all traffic at once; flip back if wrong |
| **Rollback** | Go back to the previous version |
| **Feature flag** | Deploy code switched off, turn it on separately |
| **Preview environment** | A temporary, complete copy of the system for one pull request |

### Data & queues

| Term | Plain meaning |
|---|---|
| **Partition** | Kafka splits a topic into parallel lanes; order is only guaranteed within a lane |
| **Partition key** | Decides which lane a message goes into |
| **Consumer lag** | How far behind a reader is. The single most important queue health metric. |
| **DLQ** | Dead Letter Queue — where messages go after failing too many times |
| **Backpressure** | The system telling producers "slow down, I can't keep up" |
| **Migration** | A change to the database structure |
| **PITR** | Point-In-Time Recovery — restore the database to any moment |

### Networking

| Term | Plain meaning |
|---|---|
| **DNS** | Turns a name into an address |
| **TLS / certificate** | The encryption behind HTTPS, and the proof you are who you claim |
| **ACME / Let's Encrypt** | The free, automated way to get certificates |
| **CIDR** | Notation for a block of IP addresses, e.g. `10.0.0.0/16` |
| **Ingress / egress** | Traffic coming in / going out |
| **mTLS** | Both sides prove their identity, not just the server |
| **Service mesh** | A layer that adds encryption, retries, and traffic control between services without app changes |

### Security

| Term | Plain meaning |
|---|---|
| **OIDC** | A standard for proving who you are, used to avoid stored passwords |
| **JWT** | A signed token carrying identity claims |
| **Workload identity** | A program proving what it is, without a stored secret |
| **SBOM** | A list of everything inside your container image |
| **Signing** | Cryptographic proof an artifact came from your build and wasn't altered |
| **Admission control** | The gatekeeper that refuses to let non-compliant things into the cluster |
| **Least privilege** | Give exactly the permissions needed, no more |

### Observability

| Term | Plain meaning |
|---|---|
| **Cardinality** | How many distinct label combinations a metric has. High cardinality kills Prometheus. |
| **Exemplar** | A link from a point on a metric graph to the exact trace that produced it |
| **Burn rate** | How fast you're consuming your error budget |
| **Golden signals** | Traffic, errors, latency, saturation — the four things always worth watching |

---
---

# PART 2 — THE ACTION KEY

> What to do, in what order, and why that order. This does **not** teach the technologies
> — `docs/05_STUDY_GUIDE.md` lists what to study, and the internet teaches it. This tells
> you what to *do* and why each step is where it is.

---

## 2.0 How the work is organised

Four levels:

| Level | Example | Where it lives |
|---|---|---|
| **Phase** | Phase 0 — Foundations | `docs/03_ROADMAP.md` |
| **Epic** | E0.3 — Containerisation | Azure DevOps "Feature" |
| **Story** | HEL-0.3.1 — Reference Dockerfile | Azure DevOps "User Story" |
| **Spike** | SPK-0.1 — Linux internals | A story tagged `spike` |

**For the exact file and folder structure this project ends up with** — every file, what
goes in it, which phase creates it, and why — see
**[docs/07_REPOSITORY_BLUEPRINT.md](docs/07_REPOSITORY_BLUEPRINT.md)**. This document
tells you what to do; that one tells you exactly what you'll have built once you've done
it.

A **spike** is *learning* tracked as work. That's unusual — normal companies don't ticket
"go study cgroups" — but here learning is most of the value, so it's tracked, estimated,
and has acceptance criteria.

Your board: **https://dev.azure.com/Kugleblitz712/Helios**

Work items #14 to #51 are already there, already linked so that each one shows what must
finish before it can start.

### Five rules

1. **Never work on something whose predecessors aren't done.** The board enforces this.
   The order isn't arbitrary — it's a dependency graph.
2. **Never start more than two things at once.** You have 20 hours a week. Three parallel
   threads means three unfinished things.
3. **A spike is finished when you can answer its questions out loud, from memory** — not
   when you've finished reading.
4. **If you did it by hand, it isn't done.** If it's not in a file in Git, it will be lost
   and you'll do it again.
5. **Write down anything you had to figure out twice.** That's a runbook. Runbooks are a
   deliverable, not admin.

---

## 2.1 Before you start anything

Four things, all free, none of which depend on anything else.

| # | Action | Why now |
|---|---|---|
| 1 | Create a **public** GitHub repo called `helios` | Public repos get unlimited CI minutes and a free container registry. Private caps at 2000 minutes/month, which you would exhaust. It also forces the discipline of never committing a secret — a habit best formed before you have secrets. |
| 2 | Turn on MFA on GitHub, and require a pull request to merge to `main` | You're about to put months of work here. Also, working via pull requests — even alone — is what makes the "review before merge" habit real. |
| 3 | Install **OrbStack** and remove Docker Desktop | You have 16 GB of RAM. OrbStack uses roughly half the overhead of Docker Desktop on Apple Silicon. That difference decides whether the full stack fits. |
| 4 | Create a free **Grafana Cloud** account | Later, it becomes your second monitoring backend. Proving your OpenTelemetry setup can send to two vendors at once is the point — it demonstrates you're not locked in. |

**Do not** buy a domain yet. **Do not** create an AWS account yet. **Do not** create a GCP
account yet. Free credits are clocks — starting them before you can use them wastes them.

---

## 2.2 PHASE 0 — Foundations

**Goal:** the entire Trackline product runs on your Mac, over HTTPS, in containers, started
by one command — and you can explain every part of it.

**Why this phase exists:** you cannot debug a distributed system in the cloud if you've
never seen it work locally. Everything in Phase 1 assumes you already know what "healthy"
looks like.

### Step 1 — The three standalone spikes (#30, #31, #32)

Start here because **nothing blocks them and they block other things.**

| Item | Topic | Why it's first |
|---|---|---|
| **#30** `SPK-0.3` | Defensive Bash | Every automation you write from here on is glued together with shell. Bad shell fails *silently* — a script that doesn't `set -euo pipefail` will happily continue after a failed command and report success. You need this before you write anything. |
| **#31** `SPK-0.4` | PKI and trust chains | You're about to run your own certificate authority. Understanding *why* a browser trusts a certificate now means ACME in Phase 1 and Vault in Phase 4 are familiar rather than magic. |
| **#32** `SPK-0.5` | Repository safety | Your repo is public from day one. A secret committed to a public repo is compromised permanently — deleting it doesn't help, because it's already been cloned and scraped. Learn this *before* the repo exists. |

### Step 2 — The Linux spike (#28)

The big one. Two sessions.

**Why here and not later:** every problem in this project above the operating system turns
into a Linux problem when you look closely enough. "The pod was killed" is the kernel's
OOM killer. "The service is slow" is often file descriptor exhaustion or CPU throttling
from cgroups. If Linux is opaque to you, Kubernetes is unfalsifiable magic and you'll
spend Phase 1 guessing.

It also directly blocks the containerisation epic, because **containers are just Linux
features wearing a costume** — namespaces, cgroups, and overlay filesystems. Learning
Docker before learning those means memorising commands instead of understanding.

### Step 3 — Repo, toolchain, secret scanning (#24, #25, #26)

In this order, because each depends on the previous.

| Item | Action | Why |
|---|---|---|
| **#24** | Create the repo skeleton, `.gitignore`, `.gitattributes` | The `.gitignore` must exist **before** the first commit that could contain a state file or a `.env`. Retroactive is too late. |
| **#25** | Pin every tool version in `.mise.toml` | "Works on my machine" is a bug class you can eliminate permanently, once, at the start — or debug repeatedly for months. Choose now. |
| **#26** | Install `gitleaks` as a pre-commit hook | Catches accidental secrets. **Understand that this is not a security control** — a client-side hook can be bypassed with one flag. The real control is a CI check, which comes in Phase 1. Knowing the difference is the lesson. |

### Step 4 — Container runtime baseline (#27)

Record how much memory OrbStack uses when idle.

**Why bother:** so that later, when you ask "can I run the whole observability stack
locally?", you have a number instead of a feeling. Measuring before you need to is how
capacity planning starts.

### Step 5 — The container spike (#29), then containerise everything (#33 → #36)

| Item | Action | Why in this order |
|---|---|---|
| **#29** | Study images, layers, caching, hardening | Blocks the rest. Without it you'll copy a Dockerfile from the internet and not know why it's 900 MB. |
| **#33** | Build **one** Dockerfile properly — `edge-gateway` | Get one right, then copy the pattern. Getting it wrong first replicates the mistake six times. |
| **#34** | Apply the pattern to the other five | Each language has a different trap: Node's native modules, Rust's slow compile and musl linking, Next.js standalone output. Meeting all of them is the point. |
| **#35** | Make builds work for both Intel and ARM | Your Mac is ARM; cloud nodes might be Intel. Discovering this in Phase 1 is an avoidable outage. |
| **#36** | Write a script that *checks* the image standard | A standard you don't verify is a preference, and preferences decay. This script becomes a CI gate in Phase 1 — so you're building the test before you need it. |

### Step 6 — Get the product running (#37 → #41)

| Item | Action | Why |
|---|---|---|
| **#37** | Postgres, Redis, Kafka running in Compose | Nothing else can run until state has somewhere to live. Use **real health checks**, not `sleep 10` — sleeps are how flaky startups are born. |
| **#38** | Database migrations + seed data | Wire a migration tool now, while it's easy. Retrofitting migrations into a delivery pipeline in Phase 2 is far more painful. **This is also where you'll find the bugs in my SQL** — it's never been executed. |
| **#39** | All six services wired together | The first moment the product actually exists. |
| **#40** | 500 simulated devices producing data | The system now has a pulse. This becomes your instrument for everything that follows. |
| **#41** | Point emails at a local sink (Mailpit) | So that no development email ever reaches a real person and damages your domain's sending reputation. |

### Step 7 — Your own certificate authority (#42, #43)

**Why not just use HTTP locally, like everyone does?**

Because developing on HTTP and deploying on HTTPS hides an entire class of bug — mixed
content, cookie `Secure` flags, redirect loops, WebSocket upgrades through a proxy. You
find them all at once, in production, at the worst time.

And because running your own CA teaches you the trust chain from the root down. When you
meet Let's Encrypt in Phase 1, and Vault's PKI engine in Phase 4, they'll be variations on
something you already understand.

### Step 8 — Make it reproducible (#44 → #47)

| Item | Action | Why |
|---|---|---|
| **#44** | Write the backlog sync script | Replaces my throwaway seeder with a real, idempotent one. Fitting first automation task: the thing that tracks the work becomes the work. |
| **#45** | One entrypoint — `make up`, `make down`, `make nuke` | Commands that live only in your shell history are lost knowledge. This file becomes your operational interface, and eventually the seed of Phase 7's golden path. |
| **#46** | Devcontainer | Proves the environment is defined by the repo, not by your machine's history. |
| **#47** | **Delete everything and rebuild from the README alone** | This is the real test. Every step you had to remember rather than read is a documentation defect. Fix the docs, don't fix your memory. |

### Step 9 — Close the phase (#48 → #51)

Not optional, and not paperwork.

| Item | Action | Why it matters |
|---|---|---|
| **#48** | Write an ADR on base image choice | In six months you won't remember why you chose distroless over alpine. Future-you deserves the reasoning. So does an interviewer. |
| **#49** | Draw the architecture, as code (Mermaid) | A diagram in a text file gets reviewed in a pull request. A diagram in a drawing tool rots. |
| **#50** | Retrospective | List three things that went wrong, and one concrete process change for each. "Be more careful" is not a change. |
| **#51** | Exit review | Walk the exit criteria. Answer the Phase 0 self-check questions out loud. Anything you can't answer becomes a new backlog item — not a waiver. |

**You are done with Phase 0 when a stranger could clone the repo, follow the README, and
get a working system — and when you can draw the path of one GPS point from memory.**

---

## 2.3 PHASE 1 — First Production

**Goal:** a real URL, on a real domain, with a real certificate, deployed by a pipeline,
with a dashboard and an alert.

**Buy the domain now** — a few days before you need it, because nameserver delegation and
registrar verification aren't instant and shouldn't be on the critical path.

### The order, and why

| Order | Action | Why here |
|---|---|---|
| 1 | **Build the cost controls first** — budget alarms, nightly destroy job, `make nuke` | Before any expensive resource exists. Every horror story about a $5,000 cloud bill is someone who did this second. |
| 2 | Terraform: network, then cluster, then database | **Plan your IP address ranges carefully now.** Overlapping address blocks will block cross-network connections in Phase 5 and are painful to change later. |
| 3 | Deploy the app to Kubernetes with health checks and resource limits | A container with no memory limit can starve its neighbours. A wrong liveness probe makes an overloaded service *worse* by killing pods that were merely busy. |
| 4 | DNS and automatic certificates | Now Let's Encrypt makes sense — you've already run a CA. |
| 5 | CI/CD with **OIDC**, not stored keys | Do this from the very beginning. Retrofitting identity is Phase 4's cautionary tale, and adding a stored key "temporarily" means it's there forever. |
| 6 | Prometheus, one dashboard, one alert that actually pages you | One good alert beats fifty ignored ones. |

**Deliberate failure for this phase: let a certificate expire.** On purpose. Then fix
renewal and prove it works. You will meet expired certificates in your career; better the
first time is in a lab.

---

## 2.4 PHASE 2 — GitOps and Safe Releases

**Goal:** deployments become boring.

| Order | Action | Why |
|---|---|---|
| 1 | Install Argo CD, move deployment to pull-based | Removes cluster credentials from CI. Now GitHub being compromised doesn't mean production is. |
| 2 | Split environments — dev, stage, prod — with overlays | You need somewhere to break things that isn't prod. |
| 3 | Canary deployments with **automatic** rollback driven by metrics | Humans are slow at 3am. Let the SLO decide. |
| 4 | Database migrations in the pipeline | Learn expand/contract — the only pattern that survives a rollback. If you rename a column in one step, the old version of your app breaks the moment you roll back. |
| 5 | Preview environment per pull request | The first genuinely platform-engineering thing you build. |
| 6 | Mirror one pipeline in Azure DevOps | Second CI system. Compare the models; understand what's universal and what's vendor-specific. |

**Deliberate failure: ship a bad canary.** It must roll back on its own, and you must learn
about it from an alert rather than by looking.

---

## 2.5 PHASE 3 — Observability, SLOs, and On-Call

**This is the phase that makes you an SRE rather than a DevOps engineer.** Everything
before it was building. This is operating.

| Order | Action | Why |
|---|---|---|
| 1 | OpenTelemetry in every service | Instrument once. Everything else depends on this. |
| 2 | Make one trace span the full journey — device to map | If you can do this, you can debug anything. If you can't, you're guessing. |
| 3 | Link traces to logs and metrics | One click from "this graph looks wrong" to "here's the exact request that was slow." |
| 4 | Send to a **second** backend (Grafana Cloud) | Proves vendor independence. Takes an hour and is a genuinely strong talking point. |
| 5 | Define 5 SLOs as code | Derived from what users care about, not from what's easy to measure. |
| 6 | Burn-rate alerts | Two time windows, so you catch both "everything is on fire right now" and "we're slowly bleeding budget." |
| 7 | Write a runbook for every alert | An alert with no runbook is a 3am puzzle. |
| 8 | **Actually go on call** | Put the pager on your phone. Respond to real alerts. This changes how you build things more than any other single act in this project. |

**Deliberate failure: melt Prometheus** by finding the metric with unbounded cardinality
hidden in the application code. Then fix it, and add a guard.

---

## 2.6 PHASE 4 — Security

**Goal:** nothing unverified runs; nothing authenticates with a permanent password.

| Order | Action | Why |
|---|---|---|
| 1 | Sign every image, attach an SBOM | Know what you're running and that nobody tampered with it. |
| 2 | Admission policy that **blocks** unsigned images, root containers, `:latest` | A policy that warns is a policy that's ignored. |
| 3 | Default-deny network policy | Right now any pod can talk to any pod. That's how one compromised service becomes all of them. |
| 4 | Vault issuing short-lived database credentials | Delete the static database password entirely. Not rotate it — *delete* it. |
| 5 | Ship audit logs to a SIEM | Detection requires something to detect with. |
| 6 | Create the **GCP account now** | Deliberately starting the 90-day credit clock, because Phase 5 is next. |

**Deliberate failure:** try to deploy an unsigned image running as root. Both must be
rejected, and both rejections must show up in your security logs.

---

## 2.7 PHASE 5 — Multi-Cloud and Configuration Management

| Order | Action | Why |
|---|---|---|
| 1 | AWS lab: full stack in Terraform, destroyed nightly | The forcing function for real IaC discipline. |
| 2 | Ansible builds your Mac's "on-prem" cluster from scratch | Config management only teaches you something when it manages something real. |
| 3 | Cloudflare Tunnel exposes it on your domain | Now you have a genuine hybrid cloud. |
| 4 | GCP as disaster-recovery target | Cross-cloud identity federation, which is genuinely hard and genuinely valuable. |
| 5 | Rebuild the Azure tier in Bicep as well as Terraform | Same problem, two tools. The comparison is the lesson. |
| 6 | Health-based DNS failover between clouds | The payoff for all of the above. |

**Deliberate failure: kill Azure production entirely.** Traffic must fail over. Time it
against a target you stated *in advance*.

---

## 2.8 PHASE 6 — Resilience, Scale, and Cost

| Order | Action | Why |
|---|---|---|
| 1 | Load test with k6 — load, soak, spike, breakpoint | Find the limits with numbers, not opinions. |
| 2 | Scale `device-sim` up until things break | **There are deliberate weaknesses in the application code.** Six of them. You haven't been told what they are. Finding them is the exercise. |
| 3 | Fix each one, write a postmortem for each | The postmortems are the portfolio piece. |
| 4 | Chaos experiments — with the hypothesis written *first* | Without a prior hypothesis you're just breaking things and rationalising afterwards. |
| 5 | **Restore a database backup into a fresh environment** | A backup that has never been restored is not a backup. It is a hope. |
| 6 | Autoscale on queue lag, not CPU | A consumer can sit at 2% CPU while falling an hour behind. CPU-based scaling does nothing for it. |
| 7 | Cost per customer on a dashboard | The FinOps deliverable. Now you can say what each tenant costs to serve. |

**Deliberate failure: an unannounced region loss during peak load.** Schedule it randomly
so even you don't know when.

---

## 2.9 PHASE 7 — Platform Engineering

Turn everything above into a product other engineers could use.

| Order | Action | Why |
|---|---|---|
| 1 | Backstage catalogue with owners | You can't operate what you can't inventory. |
| 2 | A template that creates a new service **with** CI, CD, dashboards, SLOs, alerts, and policies already wired | This is the single most impressive artifact in the whole project. |
| 3 | Crossplane so a developer gets a database via a pull request | Self-service instead of tickets. |
| 4 | Scorecards grading each service | Makes quality visible and comparable. |

**Deliberate failure:** hand the template to someone else and watch where they stumble.
Every stumble is a defect in your platform, not in them.

---

## 2.10 When you get stuck

In this order:

1. **Read the actual error.** Not the first line — all of it. Most of the time the answer
   is in there.
2. **Find the layer.** Is this the application, the container, Kubernetes, the network, the
   cloud, or DNS? Narrowing the layer eliminates most possibilities instantly.
3. **Check what changed.** It was working. Something changed. `git log` and your deployment
   history are the first place to look.
4. **Reproduce it smaller.** Can you make it fail with one pod? One request? `curl` instead
   of the app?
5. **Read the tool's documentation**, not a blog post. Blog posts are outdated; docs are
   versioned.
6. **Then** ask me. Bring what you tried and what you observed — that turns a question into
   a debugging session instead of a lookup.

**Budget an hour before asking. Don't budget a day.** Struggling is how it sticks;
thrashing is how you burn out.

---

## 2.11 How to tell whether you're actually learning

Not "did it work." These:

- Can you explain it to somebody else without looking?
- Can you fix it when it breaks, without the notes you wrote?
- Can you say *why* you chose this over the alternative, including the downsides?
- Could you rebuild it from scratch if it all disappeared?

Four yeses means you know it. Fewer means you followed instructions. That distinction is
exactly what an interviewer is probing for.

---

## 2.12 Traps to avoid

| Trap | Why it's fatal | What to do instead |
|---|---|---|
| Clicking in the cloud console "just this once" | It works, so you never automate it, and then you can't rebuild | If it's not in a file, it doesn't exist |
| Skipping the spikes because you want to build | You'll cargo-cult for a year and never know why anything works | The board blocks you for a reason |
| Chasing depth too early | You'll spend a month on Terraform internals and never reach Kubernetes | L2 everywhere first, L3 only on the primary tool of each category |
| Adding a tool because it's popular | You end up with 40 half-configured products and no story | Every tool needs an ADR justifying it |
| Skipping the retro | You'll repeat the same mistake in the next phase | Three things that went wrong, three concrete changes |
| Not going on call | The single biggest gap between "deployed it" and "operates it" | Put the pager on your phone |
| Treating documentation as optional | The docs *are* the deliverable that proves seniority | Write the ADR while the reasoning is fresh |
| Trying to understand everything before starting | You will never start | Understand the current phase. That's enough. |

---

## 2.13 What to do right now

1. Read Part 1 of this document properly. Once.
2. Open the board: **https://dev.azure.com/Kugleblitz712/Helios**
3. Move **#30 — Spike: Defensive Bash** to Active.
4. Do the four free setup steps in §2.1.
5. Start studying. The topics are in `docs/05_STUDY_GUIDE.md` under Phase 0.

You do not need to understand Kubernetes today. You do not need to understand
observability today. You need to understand Bash, Linux, and containers — and then the
next thing will make sense, because it's built on those.

**This is a long project. It is supposed to be.** The point isn't to finish it; it's to be
a different engineer by the time you're halfway through.
