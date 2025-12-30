# ADR-0003: Receipts (append-only + hash chain; signing-ready)

- Status: accepted
- Date: 2025-12-26
- Owners: Umbra

## Context
We must answer: "who did what, when, and why was it allowed?" with tamper-evident receipts.

## Decision
- Receipts stored append-only in Postgres.
- Each receipt includes `prev_hash` and `hash` over canonicalized content (hash chain).
- Schema includes optional fields for signatures so we can adopt signing later without refactors:
  - `signature_alg`, `signature_kid`, `signature`, `signed_at`

## Alternatives considered
- Plain audit logs: insufficient tamper evidence.
- Full signing from day 1: increases complexity and key management.

## Consequences
- Hash-chain gives immediate integrity properties within DB.
- Easy to add signing/attestation later.
