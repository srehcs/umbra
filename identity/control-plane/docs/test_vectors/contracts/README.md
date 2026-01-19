# Contract Test Vectors

These payloads define stable, cross-service expectations for API responses.
Contract tests load these vectors and assert that responses match.

## Adding a contract case
1. Add/adjust the response vector here.
2. Add a table entry in the service contract test using `testutil.RunContractSuite`.
3. Prefer typed matchers (`AssertDecisionResponse`, `AssertReceiptIngestResponse`, `AssertPEPAllowResponse`) and `AssertErrorEnvelope`.
4. Ensure OpenAPI reflects the response schema and status code.

## Placeholders
- `{{any}}`: any non-null value
- `{{uuid}}`: UUID string
- `{{nonempty}}`: non-empty string
- `{{rfc3339}}`: RFC3339 timestamp

## Files
- `pdp_decision_request.json`
- `pdp_decision_response.json`
- `pdp_error_policy_unavailable.json`
- `controlplane_receipts_error.json`
- `pep_allow_response.json`
- `pep_deny_error.json`
