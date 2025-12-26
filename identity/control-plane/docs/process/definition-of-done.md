# Definition of Done (DoD)

This DoD is **non-negotiable** and inherits from `RULES.md`. A story is “done” only when it is **shippable**.

## Required for every PR
- [ ] Builds locally (`make dev` when applicable)
- [ ] Tests added/updated (unit; integration if crossing service boundaries)
- [ ] Lint/static checks pass
- [ ] No secrets in logs; sensitive fields are redacted
- [ ] Inputs validated and bounded (size/timeouts)
- [ ] Telemetry added for new endpoints/flows:
  - traces (span)
  - structured logs
  - request/trace correlation IDs
- [ ] Rollback plan included in PR description

## Additional requirements when touching high-risk surfaces
### AuthN/AuthZ
- [ ] At least 2 approvals (per RULES)
- [ ] Negative tests for auth bypass attempts

### Multi-tenancy
- [ ] Explicit tenant scoping at storage layer
- [ ] Tests proving tenant A cannot access tenant B

### Policy semantics / PDP behavior
- [ ] Default-deny behavior tested
- [ ] “PDP unavailable” behavior documented and tested (fail-closed default)

### Receipts / audit
- [ ] Decision + invocation receipts emitted where applicable
- [ ] Hash chain fields populated
- [ ] Deterministic hashing tested

## ADR trigger
Add an ADR when you change:
- interfaces, trust boundaries, deploy model, auth, tenant model, data model, or receipt semantics.
