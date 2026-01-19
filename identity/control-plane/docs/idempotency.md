# Idempotency + Replay Protection (V0)

This document defines how Umbra uses `request_id` to provide idempotency for
receipt ingestion and internal receipt writes.

## Summary
- Receipt ingest replays return the original receipt and do not duplicate rows.
- Conflicting replays return `409 CONFLICT` with error code `CONFLICT`.
- Internal receipt writers dedupe within the configured window.
- Idempotency and receipt chains are serialized with transactional advisory locks per tenant/kind.

## Scope
- Receipt ingest (`POST /v1/receipts`) uses `request_id` as the idempotency key.
- PDP/PEP/MCP receipt writers dedupe on `request_id` to avoid duplicate receipts.
- This does **not** guarantee idempotency of upstream tool executions; clients
  must handle tool-level idempotency separately.

## Dedupe window
- Default window: 24h.
- Configurable via `UMBRA_REQUEST_ID_DEDUPE_WINDOW` (Go duration string, e.g. `24h`).
- After the window expires, the same `request_id` may be accepted as new.

## Chain lock scope
- Default scope: `tenant` (single chain per tenant/kind).
- Optional: set `UMBRA_RECEIPT_CHAIN_LOCK_SCOPE=day` to partition by UTC day.
- Partitioning improves write throughput but yields per-day chains.

## Receipt ingest behavior
- `request_id` is required in the request body.
- Replay with same `request_id` **and** identical canonical body:
  - returns `200 OK` with the original `receipt_id`/hash.
  - does not create a new receipt row.
- Replay with same `request_id` but different body:
  - returns `409 CONFLICT` with error code `CONFLICT`.
- Matching considers stored receipt fields plus the canonical body.
- Trace/span and latency/status fields are excluded from idempotency matching.

## Internal receipt writers (PDP/PEP/MCP)
- Same-window replays with identical canonical body are skipped.
- Conflicts are logged and skipped.

## Client retry rules
- Retries for the same logical action should reuse the same `request_id`.
- New actions must use a new `request_id`.
- For tool-level idempotency, use application-specific idempotency keys or
  server-side dedupe in the upstream service.
