# backend

Go backend for scale-app, a monorepo. This is a single Go module
(`scale-app/backend`) containing one or more services under `services/`.

## Services

- [`services/scale-gateway`](services/scale-gateway/README.md) — talks to a
  physical scale (Dialog 02/04 over raw TCP today, RIK planned) and exposes
  a protocol-agnostic HTTP API for sending a price and reading back a
  transaction. Runs on-site, on the scale's local network.
- [`services/core-api`](services/core-api/README.md) — the cloud backend:
  tenants/users, product/price catalog, transactions, draft/finalized
  receipts, Zitadel auth, Stripe payments.

Each service has its own `Dockerfile` (build context is this `backend/`
directory, since both share one Go module) — see the root
`docker-compose.yml`.

## Requirements

- Go 1.27 or later.
- A Postgres instance for core-api's integration tests
  (`services/core-api/internal/storage/postgres`) to run instead of skip;
  set `DATABASE_URL` to point at it. Without it, `go test ./...` still
  passes — those tests just skip.

## Common commands

Run from `backend/`:

```
go build ./...     # build everything
go vet ./...        # static checks
gofmt -l .           # list any files not gofmt-formatted (should be empty)
go test ./...        # run all unit tests
```
