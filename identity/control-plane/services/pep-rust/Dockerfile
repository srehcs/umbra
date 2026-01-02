FROM rust:1.83-bookworm AS build
WORKDIR /src
COPY services/pep-rust/Cargo.toml ./Cargo.toml
COPY services/pep-rust/src ./src
RUN cargo build --release

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /src/target/release/pep-rust /pep-rust
USER nonroot
EXPOSE 8084
ENTRYPOINT ["/pep-rust"]
