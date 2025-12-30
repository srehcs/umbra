# ADR-0006: Eventing via Redis Streams (V0 queue/event bus)

- Status: proposed (optional; not required for V0 demo)
- Date: 2025-12-26
- Owners: Umbra

## Context
We want an event bus at V0 for:
- propagating receipt events (optional)
- async processing (exports, enrichment, notifications)
We also want to remain customer-only deployable and keep ops simple.

## Decision
Use Redis Streams as an optional V0 event mechanism:
- Stream keys are tenant-scoped: `umbra:events:<tenant_id>`
- Event types:
  - `receipt.decision`
  - `receipt.invocation`
  - `policy.updated`
  - `tool.updated`

V0 writes to DB first; Streams is optional “fan-out”.

## Alternatives
- Postgres outbox pattern (durable, fewer moving parts)
- Kafka/NATS (too heavy for V0)

## Consequences
- Redis becomes an optional-but-present dependency.
- If Redis is down, DB remains source of truth; event emission can be best-effort in V0.
