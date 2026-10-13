# ADR-0004: Container base image strategy

**Status:** Accepted
**Date:** 2026-10-13

## Context

Phase 0 requires application images to run as non-root, contain no shell, handle graceful
SIGTERM, and remain below 150 MB unless a web-image exception is justified. The six
services use Go, Node.js, and Rust, so one runtime base cannot serve all of them without
adding unnecessary runtimes or shared libraries.

The web image was measured at 240,251,963 bytes on the current ARM64 environment.
`docker history` attributes 23.1 MB to the Next.js standalone application and 696 kB to
static assets; most of the remaining size comes from the Distroless Node runtime and its
OS libraries. The image serves `/dashboard`, `/map`, and `/alerts` with HTTP 200, runs as
UID 65532, has no shell, and stopped with exit 0 and `oom=false`.

## Decision

Use multi-stage builds: retain compilers and package managers in builder stages, and copy
only the executable or production runtime output into a non-root Distroless runtime image.
Select the runtime family according to the service's runtime requirements:

| Services | Runtime image | Reason |
|---|---|---|
| `edge-gateway`, `ingest`, `device-sim` | `gcr.io/distroless/static-debian12:nonroot` | These Go binaries are built with `CGO_ENABLED=0`; the static runtime avoids shipping a shell or a general-purpose OS userland. |
| `fleet-api`, `web` | `gcr.io/distroless/nodejs22-debian12:nonroot` | Both require Node.js at runtime; the web app uses Next.js standalone output to include only traced production dependencies. |
| `notifier` | `gcr.io/distroless/cc-debian12:nonroot` | The Rust executable has native shared-library dependencies, including zlib and OpenSSL, so it needs a C runtime. |

Accept the web image's measured size above 150 MB as a Phase 0 exception. The selected
Node runtime preserves the non-root and no-shell requirements, and the inspected history
shows the application payload is small relative to the runtime base. Do not treat this
exception as permission for unmeasured growth; record future size changes and revisit the
runtime choice if production pull/storage costs or the size target change.

The notifier currently copies ARM64 library paths into its runtime image. Multi-architecture
support is not implied by this decision and remains a separate implementation and
verification task.

## Alternatives considered

| Alternative | Why it was not selected |
|---|---|
| One common runtime image for all services | Would ship runtimes and libraries that some services do not need, increasing attack surface and image size. |
| `scratch` for all services | Cannot run Node.js and does not provide the shared libraries needed by the current Rust notifier; static Go images would also need explicit handling of runtime data such as CA certificates. |
| Alpine or Debian slim runtime for Node.js | These are viable, but include a shell and package-management surface that conflicts with the no-shell hardening goal unless further customized. The smaller potential image size does not outweigh that trade-off for this lab baseline. |
| Distroless `static` for Node.js or the current notifier | `static` does not contain the Node.js runtime or the notifier's required C/OpenSSL/zlib libraries. |

## Consequences

**Benefits**
- Compilers, package managers, and source code stay out of runtime images.
- Runtime images run non-root and do not include a shell.
- Static Go binaries use the smaller static runtime; Node.js and Rust receive only the runtime support they require.

**Costs and accepted risks**
- The web image exceeds the general 150 MB limit; its current measured size is 240,251,963 bytes on ARM64.
- Distroless images are harder to inspect interactively; debugging should use a separate debug build rather than adding a shell to production images.
- Rust's manually copied shared-library paths are architecture-specific and must be made target-aware before claiming multi-architecture support.
- Base-image updates and rebuilt images must be retested for startup, non-root execution, no shell, size, and graceful shutdown.
