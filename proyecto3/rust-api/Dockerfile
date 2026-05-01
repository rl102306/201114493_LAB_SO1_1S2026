# ---- Builder ----
FROM rust:1.86-alpine AS builder

RUN apk add --no-cache musl-dev

WORKDIR /app

# Cache dependencies before copying source
COPY Cargo.toml ./
RUN mkdir -p src && echo 'fn main() {}' > src/main.rs
RUN cargo build --release
RUN rm -f src/main.rs

# Build actual source
COPY src ./src
RUN touch src/main.rs && cargo build --release

# ---- Runtime ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/target/release/rust-api ./rust-api

EXPOSE 8080
CMD ["./rust-api"]
