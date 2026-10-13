# Docker Build Context, Paths, and Multi-Stage COPY

This note explains where Docker gets files during a build, how `COPY` maps those files into an image, and why a multi-stage build has more than one filesystem. The examples use Trackline's `ingest` service and its generated protobuf module.

## The Four Places To Keep Separate

When reading a Dockerfile, distinguish these locations:

1. **Host repository**: files on your Mac, such as `apps/ingest` and `apps/gen/go`.
2. **Build context**: the host directory Docker is allowed to read for `COPY`.
3. **Build-stage filesystem**: files copied into the temporary Go builder image.
4. **Runtime-stage filesystem**: files present in the final image that runs the service.

The Dockerfile is a recipe. It does not make every file in the repository available automatically. The build context is the set of host files the recipe is allowed to copy.

## The Build Context Is The Final Dot

The ingest Dockerfile needs two sibling directories:

```text
Helios/
+-- apps/                         <- use this as the build context
    +-- ingest/
    |   +-- go.mod
    |   +-- go.sum
    |   +-- cmd/ingest/
    |   +-- internal/
    |   +-- Dockerfile
    +-- gen/
        +-- go/
            +-- go.mod
            +-- go.sum
            +-- trackline/v1/*.pb.go
```

From `Helios/apps`, this command makes `apps` the context:

```sh
docker build -f ingest/Dockerfile -t ingest:local .
```

`-f ingest/Dockerfile` tells Docker where the recipe is. The final `.` tells Docker which host directory is the build context: the current directory, `apps`.

Those two paths have different jobs. The Dockerfile can be in `apps/ingest` while the context is the parent `apps` directory.

## What COPY Paths Mean

For ordinary `COPY` instructions:

- The **source** is relative to the build-context root, not relative to the Dockerfile and not relative to `WORKDIR`.
- A **relative destination** is relative to the current `WORKDIR` in the current build stage.
- An **absolute destination** starts at the root of the current build-stage filesystem.
- Docker copies files into the image being built. It does not mount the host directory at runtime.

For the command above:

```text
Host source path             COPY source      Image destination
apps/ingest/go.mod           ingest/go.mod    /src/apps/ingest/go.mod
apps/gen/go/trackline/v1/    gen/go/...       /src/apps/gen/go/trackline/v1/
```

The `COPY` source paths do not start with `apps/`, because `apps` is already the context root.

## Why These Two COPY Instructions Matter

The ingest module has this local replacement:

```go
replace github.com/helios/trackline/gen/go => ../gen/go
```

Go resolves `../gen/go` relative to the directory containing `apps/ingest/go.mod`. So, inside the builder, the expected layout is:

```text
/src/apps/
+-- ingest/
|   +-- go.mod                   # ../gen/go is resolved from here
|   +-- cmd/ingest/
|   +-- internal/
+-- gen/
    +-- go/
        +-- go.mod               # local replacement module
        +-- trackline/v1/*.pb.go
```

With `WORKDIR /src/apps/ingest`, use source paths rooted at the `apps` context and destinations that preserve this layout:

```dockerfile
WORKDIR /src/apps/ingest

# Copy module manifests first so source-only edits can reuse the dependency step.
COPY ingest/go.mod ingest/go.sum ./
COPY gen/go/go.mod gen/go/go.sum /src/apps/gen/go/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Add the source only after dependency manifests.
COPY ingest/ /src/apps/ingest/
COPY gen/go/ /src/apps/gen/go/
```

The first pair makes the module manifests available before downloading dependencies. The second pair copies the actual service and generated protobuf source. The two destinations keep the relative `../gen/go` replacement valid.

The generated `.pb.go` files may be ignored by Git and still be available to Docker: `.gitignore` controls Git, while `.dockerignore` controls which files enter a Docker build context.

## Why COPY . . Is Wrong For This Context

`COPY . .` means: copy the contents of the entire build context into the current working directory in the current stage.

With the context set to `apps` and `WORKDIR /src/apps/ingest`, this:

```dockerfile
COPY . .
```

copies the `apps` context contents under `/src/apps/ingest`. It does not selectively copy only the ingest service, and it does not preserve the intended sibling layout. The source tree can become nested under the ingest directory, so `go build ./cmd/ingest` may no longer find the package where expected.

`COPY . .` would be natural if the context were `apps/ingest`, but then Docker would be unable to read the sibling `apps/gen/go` directory. A wider context requires explicit source paths such as `ingest/` and `gen/go/`.

## Each FROM Starts A Different Stage

Every `FROM` starts a new stage with its own filesystem:

```text
Host build context: apps/
        |
        | COPY ingest/... and gen/go/...
        v
+------------------------------+
| build stage                  |
| Go compiler + source + deps  |
| go build -> /tmp/ingest      |
+------------------------------+
        |
        | COPY --from=build /tmp/ingest /ingest
        v
+------------------------------+
| runtime stage                |
| Distroless base + /ingest    |
| no Go compiler or source     |
+------------------------------+
```

In this instruction:

```dockerfile
COPY --from=build /tmp/ingest /ingest
```

`--from=build` selects the earlier stage named by `AS build`. `/tmp/ingest` is the source path inside that stage's filesystem. `/ingest` is the destination inside the current stage, which is the Distroless runtime stage.

This is not a host-to-image copy. It transfers only the compiled executable from the builder into the final image. The Go toolchain, module cache, source files, and generated source stay behind in the build stage.

## EXPOSE, Port Publishing, And Kafka

Ingest listens on `:9090` inside the container. `EXPOSE 9090` documents that container port in the image; it does not publish the port on your Mac.

To reach ingest from the host, publish a port when starting a container, for example `-p 9090:9090`. In Docker Compose, configure the port mapping there if host access is needed. Other containers on the same Compose network can usually connect to ingest by its service name and container port without host publishing.

Kafka is a separate service. `KAFKA_BROKERS=kafka:9092` is runtime configuration telling ingest where to make an outgoing connection. It does not belong in `EXPOSE` on the ingest image. Compose should provide this value and place ingest and Kafka on a network where the hostname `kafka` resolves.

## Quick Mental Checklist

When a Docker `COPY` looks confusing, ask:

1. What directory was passed as the build context (the final path argument to `docker build`)?
2. Relative to that directory, what host path does the source name identify?
3. Which `FROM` stage is currently active?
4. What is that stage's current `WORKDIR`?
5. Does the destination preserve the filesystem layout the program expects?
6. If using `--from`, is the source inside the named stage rather than on the host?