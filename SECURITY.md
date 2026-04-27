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
- No personal/local files

## Public documentation guidance
Documentation must avoid operational details that could aid abuse or expose
environment-specific information. Public docs must:
- Exclude key identifiers, KMS resource names, or environment-specific config.
- Avoid exact rotation intervals and step-by-step operational runbooks.
- Use placeholder identifiers and high-level guidance only.
