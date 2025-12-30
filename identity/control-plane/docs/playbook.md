# Engineering Playbook

This playbook is the practical companion to Umbra’s shared rules:
- `../../RULES.md`

It focuses on *how we execute* (vertical slices, review discipline, evidence), not architecture or roadmaps.

## Build strategy: vertical slices
Prefer end-to-end flows that generate receipts and telemetry over horizontal scaffolding.

V0 demo flow:
- Control Plane registers tools + policies
- PDP decides (`POST /v1/decision`)
- PEP gateway enforces + forwards
- Receipts recorded and inspected in the UI

## “Definition of Done”
See `docs/process/definition-of-done.md`.

## Local verification (minimum bar)
From `identity/control-plane/`:

```bash
make verify
```

Runs gofmt check, `go vet`, `go test ./...`, plus `pnpm` lint/build.

## PR hygiene
- Small PRs
- ADR required for interface/trust-boundary/data-model changes
- Tests + telemetry required for new endpoints/flows
