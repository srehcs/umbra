# Security Policy

## Reporting a vulnerability
Please report suspected vulnerabilities privately to the maintainers.
- Do **not** open public issues for security findings.
- Provide a clear description, impact, and reproduction steps if possible.

## Supported versions
This is an early-stage project. Only the `main` branch is supported.

## Secure development expectations
This repository follows the Umbra engineering rules in `RULES.md`.
Key requirements include:
- No secrets in git
- Dependency scanning and container scanning in CI
- Telemetry + audit receipts for security-relevant flows
