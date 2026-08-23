# backend

Go backend for scale-app, a monorepo. This is a single Go module
(`scale-app/backend`) containing one or more services under `services/`.

## Services

- [`services/scale-gateway`](services/scale-gateway/README.md) — talks to a
  physical scale (Dialog 02/04 over raw TCP today, RIK planned) and exposes
  a protocol-agnostic HTTP API for sending a price and reading back a
  transaction. Runs on-site, on the scale's local network.

More services (tenant/user management, product/price catalog, transaction
history, receipts, payment integration) will be added under `services/` as
the rest of the system is built out.

## Requirements

- Go 1.27 or later.

## Common commands

Run from `backend/`:

```
go build ./...     # build everything
go vet ./...        # static checks
gofmt -l .           # list any files not gofmt-formatted (should be empty)
go test ./...        # run all unit tests
```
