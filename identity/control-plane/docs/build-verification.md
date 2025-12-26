# Build verification (local)

Because Umbra is enterprise/security software, we treat a green local verify as the minimum bar before a PR.

## One command

From repo root:

```bash
make verify
```

This runs:
- gofmt check (fails if formatting drift)
- `go vet` + `go test ./...`
- `pnpm install` + `pnpm lint` + `pnpm build`

## Notes
- If you are offline, `pnpm install` / `go test` may fail if dependencies are not already cached.
- CI (GitHub Actions) performs the same checks on every PR/push.
