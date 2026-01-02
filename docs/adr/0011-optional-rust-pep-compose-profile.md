# ADR-0011: Optional Rust PEP via Docker Compose profile

- Status: accepted
- Date: 2026-01-02
- Owners: Umbra team

## Context
The Rust PEP is an optional service alongside the Go control plane. The default local stack (`make dev`) builds all services in the compose file, which forces Rust toolchain downloads even when a developer only needs the Go services and UI. This slows onboarding and adds friction for contributors working on non-Rust areas.

We still want a clear path to run the Rust PEP when:
- We need MCP-first enforcement behavior (protocol focus, low overhead).
- We want a second language implementation to validate cross-service contracts.
- We are testing production-oriented performance/latency characteristics for the gateway.

## Decision
Mark `pep-rust` behind a Docker Compose profile named `rust-pep`. The service is opt-in via `COMPOSE_PROFILES=rust-pep` when needed, while the default stack remains Go services + UI.

Why use the Rust PEP:
- Lower-latency, single-binary gateway for MCP tool calls.
- Easier to embed or run as a sidecar in edge deployments.
- Useful for cross-implementation parity testing with the Go PEP gateway.


## Alternatives considered
1) Keep a single compose file and rely on ad-hoc `docker compose up` service lists.
2) Split into multiple compose files (core + optional).
3) Remove Rust PEP from local stack entirely and require manual build/run.

## Consequences
- Positive: Default dev flow avoids Rust dependency downloads; optional service is explicit and documented.
- Positive: Keeps the Rust PEP available for performance testing and protocol parity.
- Negative: Developers must opt in when they want the Rust PEP.
- Follow-ups: Document `make dev-rust` in READMEs and developer docs.
