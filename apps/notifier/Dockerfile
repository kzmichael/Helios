FROM rust:1.88.0-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY Cargo.toml Cargo.lock ./

COPY src ./src

RUN cargo fetch --locked

RUN cargo build --release --locked

RUN set -eu; \
    ldd /src/target/release/notifier > /tmp/notifier-ldd; \
    mkdir -p /runtime-libs; \
    awk '$1 ~ /^lib(z|ssl|crypto)\.so/ && $3 ~ /^\// { print $1, $3 }' /tmp/notifier-ldd \
    | while read -r library_name library_path; do \
        case "$library_name" in \
            libz.so.1|libssl.so.3|libcrypto.so.3) \
                relative_path="${library_path#/}"; \
                mkdir -p "/runtime-libs/$(dirname "$relative_path")"; \
                cp -L "$library_path" "/runtime-libs/$relative_path" ;; \
        esac; \
    done


FROM gcr.io/distroless/cc-debian12:nonroot AS runtime

COPY --from=build /src/target/release/notifier /notifier

COPY --from=build /runtime-libs/ /

USER 65532:65532

ENTRYPOINT ["/notifier"]