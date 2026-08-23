# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Current state

`scale-app` is a monorepo for a market-vendor point-of-sale system: a vendor's phone talks to a physical price-computing scale (e.g. Bizerba, via its Dialog 02/04 protocol or, later, RIK) to weigh and price goods, then builds a receipt across a customer's purchases. The scale itself performs and owns the weighing/pricing calculation and its own legal-metrology certification — this system deliberately never computes or certifies a measurement itself, it only orchestrates around already-certified hardware.

Layout:
- `backend/` — Go module (`scale-app/backend`), Go 1.27+. See `backend/README.md`. Currently contains one service, `backend/services/scale-gateway` (see its own README), which speaks to the scale(s) at one location and exposes an HTTP API.
- `mobile/` — Flutter vendor app; not yet built (see `mobile/README.md` for planned screens).
- `.devcontainer/` — container for running Claude Code against this workspace (see below).

## Common commands

Run from `backend/`:
```
go build ./...    # build everything
go vet ./...       # static checks
gofmt -l .          # list unformatted files (should be empty; use gofmt -w to fix)
go test ./...       # run all unit tests
```

There is no Flutter code yet, so no Flutter commands to document — add them here once `mobile/` is scaffolded.

## Architecture notes

- The `scale-gateway` service isolates all scale-protocol-specific logic behind a `driver.ScaleDriver` interface (`internal/driver`), with wire-level framing/codecs in `internal/protocol`. Adding a new scale protocol (e.g. RIK) means implementing that interface, not changing callers. See `backend/services/scale-gateway/README.md` for the protocol details, which are inferred from vendor examples rather than an official spec and are marked as such in the code and tests — treat them as unverified until checked against real hardware or documentation.
- No legally-relevant computation lives in this codebase; the scale hardware performs and certifies the weighing/pricing calculation itself. This is a deliberate architectural choice — do not add price/weight calculation logic to the backend or mobile app.

## Devcontainer

`.devcontainer/` defines a container for running Claude Code itself against this workspace:

- Based on `node:22-slim`, installs `git`, `bash`, `ca-certificates`, `curl`, and the `@anthropic-ai/claude-code` npm package.
- Mounts the host's `~/.claude` into `/root/.claude` so OAuth login persists across container rebuilds.
- Can alternatively pass through a `CLAUDE_CODE_OAUTH_TOKEN` from the host environment instead of interactive login.
- Workspace is mounted at `/workspace` inside the container; default command drops into `bash`.
