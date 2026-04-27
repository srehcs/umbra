# Contributing to Umbra

Umbra is a security-focused codebase. Contributions are welcome, but the bar is intentionally high: changes must improve clarity, preserve trust boundaries, and leave an auditable trail.

## Before you start

- Read [README.md](README.md) for the project overview.
- Read [RULES.md](RULES.md) for engineering standards. That file is authoritative.
- Read [SECURITY.md](SECURITY.md) before reporting bugs or proposing security-sensitive changes.
- Work from the active build in `identity/control-plane/` unless a task clearly belongs elsewhere.

## Contribution principles

- Keep security boundaries explicit. Do not add flows that trust client-supplied identity, role, tenant, or signature data without server-side verification.
- Keep behavior observable. Security-relevant flows should preserve request correlation, telemetry, and receipt coverage.
- Keep contracts stable. API and schema changes should be deliberate, documented, and reflected in generated clients where applicable.
- Keep docs honest. Public docs should describe what is implemented now, what is planned, and what assumptions a reader must make.
- Keep the repo safe to publish. Never commit secrets, internal URLs, customer data, real tenant identifiers, or environment-specific operational details.

## Development workflow

From `identity/control-plane/`:

```bash
make bootstrap
make fmt
make lint
make test
make gen
```

Useful local flows:

```bash
make dev
make demo
make seed
```

If `make dev` fails because port `5432` is in use, stop the local Postgres process or remap the compose port before retrying.

## Pull request expectations

- Keep PRs focused. Small, coherent changes review faster and are easier to validate.
- Explain the risk surface. If a change touches auth, policy evaluation, receipt generation, storage, or enforcement paths, call that out explicitly.
- Add or update tests for behavioral changes.
- Update docs when public behavior, setup, or contracts change.
- Regenerate derived artifacts when needed, especially OpenAPI TypeScript outputs.
- Do not mix unrelated cleanup into a feature or security fix.

## Code standards

- Go: keep handlers thin, propagate `context.Context`, return typed errors, and follow standard formatting.
- TypeScript: keep strict typing, centralize API access, and validate untrusted inputs at boundaries.
- UI: prefer shared components under `identity/control-plane/ui/components/ui/`.
- Storage and query paths: prefer indexed, DB-side filtering and pagination over in-memory processing.

## Security reporting

Do not open public issues for vulnerabilities. Follow the process in [SECURITY.md](SECURITY.md) and report suspected vulnerabilities privately to the maintainers.

## Documentation and demos

- Keep screenshots, examples, and walkthroughs aligned with the current product state.
- Use placeholders instead of real secrets, key identifiers, cert paths, or tenant data.
- Avoid overstating production readiness. If a feature is local-only or partially implemented, say so directly.

## License

By submitting a contribution, you agree that your contribution will be licensed under the Apache License 2.0 in this repository.
