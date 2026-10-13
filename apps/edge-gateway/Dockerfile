# syntax=docker/dockerfile:1

FROM golang:1.25.12-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -o /tmp/edge-gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /tmp/edge-gateway /edge-gateway

EXPOSE 8080
ENTRYPOINT [ "/edge-gateway" ]