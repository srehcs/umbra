# Release Train / Demo Cuts

We run a lightweight release train to keep demos and pilots stable.

## Cadence (recommended)
- **Daily:** merge small PRs to `main` (trunk-based)
- **Weekly:** create a “demo cut” tag (e.g., `demo-YYYY-MM-DD`)
- **Milestones:** only when we hit a coherent vertical slice

## Rules for a demo cut
A demo cut is tagged only when:
- CI green (tests/lint/security scans)
- Local stack works (`make dev`)
- Demo runbook passes end-to-end (`docs/runbooks/demo.md`)
- No “known broken” features without explicit notes

## Emergency fixes
- Fix-forward is preferred
- If a revert is needed:
  - revert must be accompanied by a brief incident note in PR
  - add regression test to prevent recurrence
