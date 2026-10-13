# syntax=docker/dockerfile:1

FROM golang:1.25.12-alpine AS build
WORKDIR /src/apps/device-sim

COPY device-sim/go.mod device-sim/go.sum ./
COPY gen/go/go.mod gen/go/go.sum /src/apps/gen/go/

RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY device-sim/ ./
COPY gen/go/ /src/apps/gen/go/


RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -o /tmp/device-sim ./cmd/simulator

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /tmp/device-sim /device-sim

ENTRYPOINT [ "/device-sim" ]