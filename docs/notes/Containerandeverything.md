# Container Image Engineering

> **Core principle:** A production container image is not just "an application packaged into a Dockerfile."
>
> It is a carefully constructed filesystem made of layers, optimized around build caching, separated into build/runtime environments, built for the correct architecture, and restricted to the privileges and runtime dependencies the application actually needs.

---

# 1. The Container Image Mental Model

A Docker image can be thought of as:

```text
                    Docker Image
                         │
             ┌───────────┴───────────┐
             │                       │
        Immutable layers        Image metadata
             │
      ┌──────┴──────┐
      │             │
   Base image   Application
                layers
```

When a container starts:

```text
              Container
                  │
          ┌───────┴────────┐
          │                │
      Image layers    Writable layer
          │                │
          └───────┬────────┘
                  ▼
          Unified filesystem
```

The container sees the layers as one filesystem.

This connects directly to the Linux concepts previously studied:

```text
Namespaces → what the process can see
cgroups    → what resources it can use
OverlayFS  → what filesystem it sees
```

---

# 2. Image Layers

Consider:

```dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .

RUN pip install -r requirements.txt

COPY app.py .

CMD ["python", "app.py"]
```

Conceptually:

```text
Layer 1
────────────────
Python base image

Layer 2
────────────────
/app

Layer 3
────────────────
requirements.txt

Layer 4
────────────────
Python dependencies

Layer 5
────────────────
app.py

Metadata
────────────────
CMD
```

Each instruction can contribute to the resulting image's filesystem state.

The exact internal storage behavior depends on the builder/runtime, but the layered model is the important mental model.

---

# 3. Why Layers Exist

Suppose we have:

```text
Base image
    ↓
Python dependencies
    ↓
Application
```

If only the application changes:

```text
Base image          → REUSE
Dependencies        → REUSE
Application         → REBUILD
```

We don't need to rebuild everything from scratch.

This gives us:

* Faster builds
* Layer reuse
* Less network transfer
* Better CI/CD performance
* Efficient image distribution

---

# 4. Union Filesystem

The Linux filesystem used by container runtimes can combine multiple layers into a unified view.

Conceptually:

```text
          Application sees
                 │
                 ▼
        ┌─────────────────┐
        │     merged      │
        └─────────────────┘
                 ▲
        ┌─────────────────┐
        │   upper layer   │
        └─────────────────┘
                 ▲
        ┌─────────────────┐
        │   lower layer   │
        └─────────────────┘
```

Multiple layers appear as one filesystem.

This is the **union filesystem** concept.

OverlayFS is one Linux implementation used for this type of layered filesystem.

---

# 5. Image vs Container

This distinction is critical.

## Image

Mostly immutable:

```text
Image
├── Base layer
├── Dependency layer
└── Application layer
```

## Container

An image plus a writable runtime layer:

```text
Container
├── Base layer
├── Dependency layer
├── Application layer
└── Writable container layer
```

If the application writes:

```text
/tmp/file.txt
```

the original image isn't modified.

The write goes into the container's writable layer.

---

# 6. Copy-on-Write

Suppose an image contains:

```text
/app/config.yaml
```

The container modifies it.

Conceptually:

```text
Image layer
    │
    │ original config.yaml
    ▼
Container writable layer
    │
    │ modified config.yaml
    ▼
Merged filesystem
```

The original image remains unchanged.

This behavior is commonly associated with **copy-on-write** and OverlayFS copy-up behavior.

---

# 7. Docker Build Cache

Docker evaluates a Dockerfile sequentially.

Example:

```dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .

RUN pip install -r requirements.txt

COPY app.py .

CMD ["python", "app.py"]
```

Think of the build as:

```text
FROM
 ↓
WORKDIR
 ↓
COPY requirements.txt
 ↓
RUN pip install
 ↓
COPY app.py
 ↓
CMD
```

Docker attempts to reuse previously built results where the relevant inputs have not changed.

---

# 8. Cache Hit Example

First build:

```text
FROM                  BUILD
WORKDIR               BUILD
COPY requirements     BUILD
RUN pip install       BUILD
COPY app.py           BUILD
```

Now change only:

```text
app.py
```

Next build can conceptually become:

```text
FROM                  CACHE
WORKDIR               CACHE
COPY requirements     CACHE
RUN pip install       CACHE
COPY app.py           REBUILD
```

This is exactly what we want.

---

# 9. Cache Invalidation

Consider:

```text
A
↓
B
↓
C
↓
D
↓
E
```

If `C` changes:

```text
A → CACHE
B → CACHE
C → REBUILD
D → REBUILD
E → REBUILD
```

The important principle is:

> **Once a relevant build step is invalidated, subsequent dependent steps generally need to be rebuilt.**

Therefore Dockerfile ordering matters.

---

# 10. Bad Dockerfile

```dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY . .

RUN pip install -r requirements.txt

CMD ["python", "app.py"]
```

Problem:

```text
COPY .
  ↓
application changes
  ↓
cache invalidated
  ↓
pip install runs again
```

A tiny source-code change can cause dependency installation to run again.

---

# 11. Better Dockerfile

```dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .

RUN pip install -r requirements.txt

COPY . .

CMD ["python", "app.py"]
```

Now:

```text
requirements.txt
       ↓
pip install
       ↓
application code
```

Changing application code doesn't normally invalidate the dependency layer.

---

# 12. Cache Invalidation Order

A useful design principle:

```text
LEAST FREQUENTLY CHANGING
            │
            ▼
       Base image
            │
            ▼
    OS dependencies
            │
            ▼
 Language dependencies
            │
            ▼
   Application code
            │
            ▼
MOST FREQUENTLY CHANGING
```

Example:

```dockerfile
FROM python:3.12-slim

RUN apt-get update && \
    apt-get install -y curl && \
    rm -rf /var/lib/apt/lists/*

COPY requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

COPY src/ ./src/

CMD ["python", "src/app.py"]
```

The structure intentionally maximizes cache reuse.

---

# 13. Build Context

When running:

```bash
docker build -t my-api .
```

the final `.` means:

```text
current directory = build context
```

Docker can potentially send files in that context to the builder.

Therefore use:

```text
.dockerignore
```

Example:

```text
.git
.gitignore
__pycache__
*.pyc
.env
node_modules
tests
README.md
```

Why?

Because unnecessary files can:

* Increase build context
* Slow builds
* Cause unnecessary cache invalidation
* Accidentally expose sensitive files to the build

---

# 14. Multi-Stage Builds

A build environment often needs more software than the production environment.

Example:

```text
BUILD
├── compiler
├── build tools
├── source code
├── development dependencies
└── application
```

Production might only need:

```text
RUNTIME
├── runtime libraries
└── application
```

Multi-stage builds separate these environments.

---

# 15. Multi-Stage Example

Go application:

```dockerfile
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o myapp .
```

Then:

```dockerfile
FROM debian:bookworm-slim

COPY --from=builder /app/myapp /usr/local/bin/myapp

CMD ["myapp"]
```

Conceptually:

```text
             BUILD STAGE
        ┌──────────────────┐
        │ Go compiler      │
        │ Go dependencies  │
        │ Source code      │
        │ Build tools      │
        └────────┬─────────┘
                 │
                 │ compile
                 ▼
              myapp
                 │
                 ▼
             RUNTIME STAGE
        ┌──────────────────┐
        │ Debian slim      │
        │ myapp            │
        └──────────────────┘
```

The compiler doesn't need to be shipped to production.

---

# 16. Benefits of Multi-Stage Builds

Without multi-stage:

```text
Production image
├── compiler
├── source
├── build tools
├── development dependencies
└── application
```

With multi-stage:

```text
Production image
├── runtime
└── application
```

Benefits:

* Smaller image
* Smaller attack surface
* Less unnecessary software
* Cleaner runtime
* Better separation between build and execution

---

# 17. Base Image Choices

Common base-image categories:

```text
scratch
distroless
alpine
debian-slim
```

Do not think:

```text
smallest = automatically best
```

Instead evaluate:

```text
Compatibility
Security
Debuggability
Size
Libraries
Package availability
Operational requirements
```

---

# 18. scratch

`scratch` is effectively an empty image starting point.

Example:

```dockerfile
FROM scratch

COPY myapp /myapp

ENTRYPOINT ["/myapp"]
```

Result:

```text
scratch
   +
application binary
```

Very small.

But it generally doesn't provide:

```text
Shell
Package manager
Normal userspace utilities
Standard OS libraries
```

Therefore it is appropriate only when the application can operate without those components.

---

# 19. Distroless

Distroless images aim to contain the runtime components required by the application without providing a conventional general-purpose Linux userspace.

Conceptually:

```text
Distroless
├── required runtime components
├── required libraries/data
└── application
```

Typically:

```text
No shell
No package manager
Minimal debugging tools
```

Advantages:

```text
Small runtime
Reduced software surface
Less unnecessary tooling
```

Trade-off:

```text
Harder interactive debugging
```

For example:

```bash
docker exec -it container sh
```

may not work because there is no shell.

---

# 20. Alpine

Alpine is a small Linux distribution.

Typical characteristics:

```text
Small
Shell available
Package manager available
musl libc
```

Example:

```dockerfile
FROM alpine:3.x
```

Advantages:

* Small footprint
* Package manager
* Shell
* Useful minimal userspace

Important consideration:

```text
Alpine → musl
```

rather than glibc.

---

# 21. Debian Slim

Example:

```dockerfile
FROM debian:bookworm-slim
```

It provides a relatively conventional Debian userspace while reducing some of the size of a full Debian image.

Typical characteristics:

```text
glibc
apt
familiar Linux environment
broad software compatibility
```

It is often a practical choice when compatibility and operational familiarity matter more than minimizing every megabyte.

---

# 22. Base Image Comparison

| Base        | Size tendency | Shell      | Package manager | libc               | Typical use                      |
| ----------- | ------------- | ---------- | --------------- | ------------------ | -------------------------------- |
| `scratch`   | Tiny          | No         | No              | None by itself     | Self-contained binaries          |
| Distroless  | Very small    | Usually no | No              | Depends on variant | Minimal production runtime       |
| Alpine      | Small         | Yes        | Yes             | musl               | Small Linux runtime              |
| Debian slim | Larger        | Yes        | Yes             | glibc              | Compatibility/familiar userspace |

The correct choice depends on the application.

---

# 23. musl vs glibc

Two important Linux C libraries:

```text
glibc
musl
```

Typical examples:

```text
Debian / Ubuntu → glibc
Alpine          → musl
```

This matters because applications and native libraries can depend on libc behavior and ABI compatibility.

---

# 24. Why libc Matters

Consider:

```text
Python application
       │
       ▼
Python package
       │
       ▼
Native C/C++ extension
       │
       ▼
libc
```

A native dependency built/tested against glibc may not behave identically in a musl environment.

Possible problems can include:

```text
Build failures
Missing symbols
Import errors
Binary execution failures
Unexpected runtime behavior
```

Not every application has these problems.

The important point is:

> **Alpine is not simply "Debian but smaller." It uses a different libc implementation.**

---

# 25. Example: Python

Compare:

```dockerfile
FROM python:3.12-slim
```

with:

```dockerfile
FROM python:3.12-alpine
```

The difference isn't merely image size.

The userspace environment differs:

```text
python:3.12-slim
        ↓
Debian-based
        ↓
glibc
```

versus:

```text
python:3.12-alpine
        ↓
Alpine-based
        ↓
musl
```

This can matter for packages containing native extensions.

---

# 26. When to Think About musl vs glibc

Pay particular attention when using:

* Native Python packages
* C/C++ libraries
* Precompiled binaries
* Vendor software
* Native language runtimes
* System libraries
* Database/client libraries
* Software distributed as architecture-specific binaries

The question should be:

> **Does my dependency stack support the libc environment I'm choosing?**

Not:

> "Is Alpine better?"

---

# 27. Linux Capabilities

Linux historically associates significant privileges with root.

Modern Linux also divides many privileges into smaller units called **capabilities**.

Examples:

```text
CAP_NET_BIND_SERVICE
CAP_NET_ADMIN
CAP_SYS_ADMIN
CAP_CHOWN
CAP_SETUID
```

Think:

```text
Root privileges
      │
      ├── Capability A
      ├── Capability B
      ├── Capability C
      └── ...
```

This allows more granular privilege management.

---

# 28. Why Capabilities Matter in Containers

Suppose an application needs to bind to a privileged network port.

Instead of giving the entire container broad privileges, a specific capability can sometimes provide the required operation:

```text
CAP_NET_BIND_SERVICE
```

The security principle is:

> **Give a process only the privileges it actually needs.**

---

# 29. Dropping Capabilities

A hardened container can start from:

```bash
docker run --cap-drop=ALL ...
```

and selectively add a required capability:

```bash
docker run \
  --cap-drop=ALL \
  --cap-add=NET_BIND_SERVICE \
  ...
```

The exact capability requirements should be determined from the application's behavior rather than added blindly.

---

# 30. CAP_SYS_ADMIN

One particularly important capability:

```text
CAP_SYS_ADMIN
```

It covers a broad range of administrative operations.

Therefore:

> Do not add `CAP_SYS_ADMIN` simply because "something isn't working."

First determine which kernel operation is failing and what privilege is actually required.

---

# 31. Principle of Least Privilege

The overall security model:

```text
Default
   ↓
Drop unnecessary privileges
   ↓
Drop unnecessary capabilities
   ↓
Run as non-root where possible
   ↓
Add only explicitly required privileges
```

This reduces the impact of a compromised application.

---

# 32. Multi-Architecture Images

Applications may run on different CPU architectures.

Common platforms:

```text
linux/amd64
linux/arm64
linux/arm/v7
```

For example:

```text
Intel/AMD server
    ↓
amd64

Apple Silicon / ARM server
    ↓
arm64
```

A binary compiled for one architecture generally cannot simply be executed natively on another architecture.

---

# 33. The Problem

Imagine:

```text
Developer laptop
    ↓
ARM64
```

but:

```text
Production server
    ↓
AMD64
```

If you build only an ARM64 image:

```text
Local
  ↓
WORKS

Production
  ↓
FAILS
```

This is why architecture must be considered during image creation.

---

# 34. Image Manifest / Image Index

A registry can associate one image tag with multiple architecture-specific images.

Conceptually:

```text
                 my-api:1.0
                     │
                     ▼
              Image Index
                /       \
               /         \
              ▼           ▼
         linux/amd64   linux/arm64
              │           │
              ▼           ▼
           Image A      Image B
```

When the client pulls:

```bash
docker pull my-api:1.0
```

the tooling can select the appropriate platform image.

---

# 35. Multi-Architecture Build

Docker Buildx can build multiple platforms:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t myregistry/my-api:1.0 \
  --push .
```

Conceptually:

```text
                  Dockerfile
                      │
             ┌────────┴────────┐
             │                 │
             ▼                 ▼
         AMD64 build       ARM64 build
             │                 │
             ▼                 ▼
          Image A           Image B
             │                 │
             └────────┬────────┘
                      ▼
                 Image Index
                      │
                      ▼
                 my-api:1.0
```

---

# 36. Why Multi-Arch Matters in DevOps

Modern environments can contain:

```text
Developer machines
       ↓
ARM64 / AMD64

CI runners
       ↓
AMD64 / ARM64

Kubernetes nodes
       ↓
AMD64 / ARM64
```

Therefore production image design should explicitly consider supported architectures.

---

# 37. Complete Example — Python API

Project:

```text
my-api/
├── app.py
├── requirements.txt
├── .dockerignore
└── Dockerfile
```

Dockerfile:

```dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

RUN useradd --create-home appuser

USER appuser

CMD ["python", "app.py"]
```

Analyze it.

### Layering

```text
Python base
     ↓
Working directory
     ↓
requirements.txt
     ↓
Python dependencies
     ↓
app.py
     ↓
runtime user configuration
```

### Cache

Changing:

```text
app.py
```

should not require reinstalling dependencies.

### Security

The application runs as:

```text
appuser
```

rather than root.

### Base image

We selected:

```text
python:3.12-slim
```

which gives us a relatively conventional Debian/glibc environment.

---

# 38. Complete Example — Multi-Stage Go

```dockerfile
FROM golang:1.24 AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/myapp .
```

Runtime:

```dockerfile
FROM scratch

COPY --from=builder /out/myapp /myapp

ENTRYPOINT ["/myapp"]
```

Architecture:

```text
Builder
   │
   ├── compiler
   ├── dependencies
   └── source
   │
   ▼
Binary
   │
   ▼
scratch
   │
   ▼
Minimal runtime image
```

But before using `scratch`, verify:

```text
Does the binary need libc?
Does it need CA certificates?
Does it need timezone data?
Does it write files?
Does it need a shell?
Does it need other runtime files?
```

A small image is useful only if it still satisfies the application's runtime requirements.

---

# 39. Putting Everything Together

Container image engineering can be viewed as:

```text
                       Dockerfile
                           │
                           ▼
                     Build process
                           │
              ┌────────────┼────────────┐
              │            │            │
           Layers       Cache       Multi-stage
              │            │            │
              └────────────┼────────────┘
                           ▼
                      Final image
                           │
             ┌─────────────┼─────────────┐
             │             │             │
         Base image   Capabilities   Architecture
             │             │             │
        ┌────┴────┐        │       ┌─────┴─────┐
        │         │        │       │           │
      musl      glibc      │     amd64       arm64
        │         │        │       │           │
        └────┬────┘        │       └─────┬─────┘
             │             │             │
             └─────────────┼─────────────┘
                           ▼
                       Container
                           │
                           ▼
                     Linux kernel
```

---

# 40. Production Image Design Questions

Whenever you write a Dockerfile, ask these questions.

## Layers

```text
What layers am I creating?
```

## Cache

```text
What changes frequently?
What changes rarely?
Can I structure the Dockerfile to maximize cache reuse?
```

## Build

```text
What is needed only to BUILD the application?
```

## Runtime

```text
What is actually required to RUN the application?
```

## Base image

```text
Do I need:
- shell?
- package manager?
- debugging tools?
- glibc?
- musl?
- certificates?
- timezone data?
```

## Security

```text
Can I run as non-root?
Which capabilities are actually required?
Can unnecessary capabilities be dropped?
```

## Architecture

```text
Do I need:
- amd64?
- arm64?
- both?
```

---

# 41. Troubleshooting Based on These Concepts

## Problem: Image is huge

Investigate:

```text
Unnecessary dependencies
Build tools in runtime image
Large base image
Poor multi-stage design
Unnecessary files in build context
```

---

## Problem: Build is slow

Investigate:

```text
Poor Dockerfile ordering
Cache invalidation
Large build context
Unnecessary COPY operations
Dependency installation occurring too often
```

---

## Problem: Application works on Debian but not Alpine

Investigate:

```text
musl vs glibc
Native dependencies
Precompiled binaries
ABI compatibility
Build dependencies
```

---

## Problem: Container cannot perform an operation

Investigate:

```text
User privileges
Linux capabilities
Filesystem permissions
Seccomp/security restrictions
Application requirements
```

Don't immediately do:

```bash
--privileged
```

Instead determine exactly what operation requires additional permission.

---

## Problem: Image works on Mac but not production

Investigate:

```text
CPU architecture
amd64 vs arm64
native binaries
multi-architecture image
```

---

# 42. Final Mental Model

Memorize this:

```text
                 CONTAINER IMAGE
                       │
                       ▼
                    LAYERS
                       │
                       ▼
               UNION FILESYSTEM
                       │
                       ▼
                  BUILD CACHE
                       │
          ┌────────────┴────────────┐
          │                         │
     Build stage              Runtime stage
          │                         │
     compiler/tools          minimal runtime
          │                         │
          └────────────┬────────────┘
                       ▼
                  BASE IMAGE
                       │
              ┌────────┼────────┐
              │        │        │
            Alpine   Debian   Distroless
              │        │
            musl     glibc
                       │
                       ▼
                  APPLICATION
                       │
                       ▼
                 CAPABILITIES
                       │
                       ▼
                 ARCHITECTURE
                 /          \
              amd64        arm64
```

---

# 43. The Eight Things to Remember

### 1. Layers

```text
Image = multiple filesystem layers
```

### 2. Union filesystem

```text
Multiple layers → one filesystem view
```

### 3. Cache

```text
Stable → early
Changing → late
```

### 4. Multi-stage

```text
Build environment ≠ Runtime environment
```

### 5. Base images

```text
scratch
distroless
alpine
debian-slim
```

Choose based on requirements, not just size.

### 6. libc

```text
Alpine → musl
Debian → glibc
```

Native dependencies can make this important.

### 7. Capabilities

```text
Don't give broad privileges when a narrow capability is enough.
```

### 8. Multi-architecture

```text
One tag
   ↓
Image index
   ├── amd64
   └── arm64
```

---

# 44. The Core DevOps Mental Model

When you see a Dockerfile, don't read it merely as syntax.

Think:

```text
What filesystem layers am I creating?
              ↓
What can be cached?
              ↓
What invalidates that cache?
              ↓
What is needed only during BUILD?
              ↓
What is needed during RUNTIME?
              ↓
Which base userspace do I need?
              ↓
musl or glibc?
              ↓
Which privileges does the process need?
              ↓
Which CPU architectures do I support?
              ↓
What should the final production image contain?
```

> **Goal:** Move from "I know how to write a Dockerfile" to **"I understand why this container image is designed this way."**



┌─────────────────────────────────────────────────────────────┐
│              CONTAINER IMAGE CHEAT SHEET                    │
└─────────────────────────────────────────────────────────────┘

1. IMAGE LAYERS
────────────────
Image = stack of mostly immutable filesystem layers.

Example:
FROM base
   ↓
dependencies
   ↓
application
   ↓
final image

Container = Image + writable runtime layer


2. UNION FILESYSTEM
───────────────────
Multiple layers → one filesystem view.

          Container filesystem
                  ↑
          ┌───────────────┐
          │ Writable      │
          ├───────────────┤
          │ App layer     │
          ├───────────────┤
          │ Dependencies  │
          ├───────────────┤
          │ Base image    │
          └───────────────┘

OverlayFS is a Linux mechanism commonly used for this.


3. BUILD CACHE
──────────────
Docker builds instructions sequentially.

A change can invalidate later dependent steps.

Good:

FROM python:3.12-slim
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .

Bad:

FROM python:3.12-slim
COPY . .
RUN pip install -r requirements.txt

RULE:
Stable things → EARLY
Frequently changing things → LATE


4. CACHE INVALIDATION
─────────────────────
A → B → C → D → E

If C changes:

A → CACHE
B → CACHE
C → REBUILD
D → REBUILD
E → REBUILD


5. MULTI-STAGE BUILDS
─────────────────────
BUILD environment ≠ RUNTIME environment

Builder:
compiler + source + build tools
              ↓
           binary
              ↓
Runtime:
runtime + binary

Goal:
Don't ship compilers/build tools into production.


6. BASE IMAGES
──────────────
scratch
  → tiniest
  → no shell/package manager
  → suitable for self-contained binaries

distroless
  → minimal runtime
  → usually no shell/package manager
  → smaller attack surface

alpine
  → small
  → shell/package manager
  → musl libc

debian-slim
  → larger
  → familiar userspace
  → glibc
  → broad compatibility

RULE:
Smallest ≠ automatically best.


7. musl vs glibc
────────────────
Debian/Ubuntu → usually glibc
Alpine       → musl

Important for:
• native libraries
• compiled extensions
• prebuilt binaries
• vendor software
• ABI compatibility

Think:
"Does my dependency stack support this libc?"


8. LINUX CAPABILITIES
─────────────────────
Root privileges are divided into capabilities.

Examples:
CAP_NET_BIND_SERVICE
CAP_NET_ADMIN
CAP_SYS_ADMIN
CAP_CHOWN
CAP_SETUID

Principle:
Least privilege.

Prefer:
--cap-drop=ALL
      +
only required --cap-add


9. MULTI-ARCH
─────────────
One tag can point to multiple platform-specific images.

my-api:1.0
     │
     ▼
Image Index
   ├── linux/amd64
   └── linux/arm64

Build example:

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t my-api:1.0 \
  --push .


10. .dockerignore
─────────────────
Keep unnecessary files out of build context.

Common:
.git
.env
__pycache__
*.pyc
node_modules


11. PRODUCTION CHECKLIST
────────────────────────
Before shipping an image:

□ Can layers be cached efficiently?
□ Are frequently changing files late?
□ Can build tools stay out of runtime?
□ Is the base image appropriate?
□ Does the app need glibc or musl?
□ Can the app run as non-root?
□ Can unnecessary capabilities be dropped?
□ Are secrets excluded?
□ Is the image built for required architectures?
□ Does the runtime actually contain everything the app needs?


12. TROUBLESHOOTING MAP
───────────────────────
Huge image
→ base image
→ unnecessary dependencies
→ build tools
→ multi-stage build

Slow build
→ cache invalidation
→ Dockerfile ordering
→ large build context
→ .dockerignore

Works on Debian, fails on Alpine
→ musl vs glibc
→ native dependencies
→ ABI compatibility

Permission denied
→ user
→ filesystem permissions
→ capabilities

Works on ARM Mac, fails on server
→ amd64 vs arm64
→ architecture-specific binary/image

Container can't exec "sh"
→ probably minimal/distroless/scratch image


13. ONE-LINE MENTAL MODEL
─────────────────────────

Dockerfile
   ↓
Layers
   ↓
Cache
   ↓
Build stage
   ↓
Runtime stage
   ↓
Base image
   ↓
libc (musl/glibc)
   ↓
Least privilege
   ↓
Target architectures
   ↓
Production container