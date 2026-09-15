FROM rust:1.90-bookworm AS build
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY crates ./crates
RUN cargo build --release --workspace --locked && cargo test --workspace --locked
FROM debian:bookworm-slim
RUN useradd --uid 10001 --create-home vb
COPY --from=build /src/target/release/rent-service /usr/local/bin/rent-service
COPY --from=build /src/target/release/rent-core /usr/local/bin/rent-core
USER 10001:10001
EXPOSE 8081
CMD ["rent-service"]
