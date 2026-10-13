# syntax=docker/dockerfile:1

FROM golang:1.25.12-alpine AS build

# Build with `apps/` as the context so this service and its sibling generated module are available.
WORKDIR /src/apps/ingest

COPY ingest/go.mod ingest/go.sum ./
COPY gen/go/go.mod gen/go/go.sum /src/apps/gen/go/

RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Keep the generated module at ../gen/go, as required by the replace directive in go.mod.
COPY ingest/ ./
COPY gen/go/ /src/apps/gen/go/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -o /tmp/ingest ./cmd/ingest

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /tmp/ingest /ingest

EXPOSE 9090
ENTRYPOINT ["/ingest"]