# Risk Register (Top Risks)

This is the working list of the highest risks for V0-C. Update as we learn.

## R1: Cross-tenant data leakage (highest severity)
- **Impact:** catastrophic
- **Mitigation:** tenant_id required in every query; tenant tests; optional RLS plan in `docs/security/rls_plan.md`

## R2: Auth bypass / incorrect tenant derivation
- **Impact:** catastrophic
- **Mitigation:** Keycloak OIDC validation; deny on missing/invalid tenant; audit changes; negative tests

## R3: PDP unavailable → unsafe allow
- **Impact:** high
- **Mitigation:** fail closed default; explicit exceptions only via ADR; circuit-breakers + timeouts

## R4: Receipt tampering / lack of audit integrity
- **Impact:** high
- **Mitigation:** append-only receipts; hash chain; signing-ready schema; tests verify chain properties

## R5: Logging secrets / sensitive data leakage
- **Impact:** high
- **Mitigation:** redaction policy; structured logs; tests for “never log” fields

## R6: Policy evaluation bugs (privilege escalation)
- **Impact:** high
- **Mitigation:** default-deny; high coverage for evaluator; negative tests for bypass

## R7: Replay attacks against tool invocations
- **Impact:** medium/high
- **Mitigation:** include nonce/timestamp in receipts; optional request signatures later; rate limiting

## R8: Performance bottlenecks (PEP latency)
- **Impact:** medium
- **Mitigation:** Redis caching; bounded timeouts; profiling; RED metrics

## R9: Supply-chain compromise (deps/build)
- **Impact:** high
- **Mitigation:** CI scans (OSV, Trivy, gitleaks); provenance stub now; enable keyless later

## R10: Inconsistent contracts (docs drift from code)
- **Impact:** medium
- **Mitigation:** OpenAPI-first; generated clients; CI drift checks
