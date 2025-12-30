# ADR-0005: Policy evaluator interface (ABAC now, swappable later)

- Status: accepted
- Date: 2025-12-26
- Owners: Umbra

## Context
We need policy evaluation semantics immediately (V0), but we want the ability to migrate to
OPA/bundles or richer models later without rewriting the entire PDP.

## Decision
Define a stable `Evaluator` interface in the PDP:
- Input: a normalized `DecisionInput` (tenant, actor, tool, resource, context)
- Output: `DecisionResult` (allow/deny + obligations + reason + policy metadata)

V0 implementation:
- in-code ABAC rules stored in Postgres (JSON rules)

Later implementations:
- OPA embedded (Rego)
- OPA sidecar (bundle fetch + eval)
- Relationship-based model (Zanzibar-like) if required

## Alternatives
- Bake ABAC into the HTTP handler: fastest but painful to evolve
- Start with OPA: higher overhead for V0 and more moving parts

## Consequences
- Slight upfront abstraction cost
- Much easier migration path to more advanced policy engines
