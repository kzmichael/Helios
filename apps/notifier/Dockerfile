FROM rust:1.88.0-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY Cargo.toml Cargo.lock ./

COPY src ./src

RUN cargo fetch --locked

RUN cargo build --release --locked

FROM gcr.io/distroless/cc-debian12:nonroot AS runtime

COPY --from=build /src/target/release/notifier /notifier

USER 65532:65532

ENTRYPOINT ["/notifier"]