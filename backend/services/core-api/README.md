# core-api

The cloud backend for scale-app: tenants (market vendors), their users
(admin/vendor), the product/price catalog, transactions produced by scale
weigh events, and the draft/finalized receipts built from them.

This service never computes or certifies a measurement itself — see the
top-level README for why. It's pure orchestration around already-approved
data coming from `scale-gateway`.

## Layout

- `internal/domain` — plain business types (Tenant, User, Product,
  Transaction, Receipt). `Receipt` carries its own state machine
  (`AddLine`/`RemoveLine` only while a draft, `Finalize` locks it).
- `internal/storage` — the `storage.Store` persistence contract, with a
  `memory` implementation (fast unit tests) and a `postgres` implementation
  (pgx, pure Go, embedded schema in `internal/storage/postgres/schema.sql`).
- `internal/auth` — Zitadel integration. `ZitadelVerifier` verifies OIDC/JWT
  access tokens; `ZitadelAdminClient` creates/deletes vendor users via
  Zitadel's v2 User API. **Tenant scoping and role authorization come from
  core-api's own `User` record** (looked up by the token's subject), not
  from Zitadel claims — see the package doc comment for why.
- `internal/receipt` — renders a finalized receipt to text/HTML and sends it
  by email (`SMTPSender`, stdlib `net/smtp`). No printer hardware
  integration; `RenderText` produces the content a print pathway would send
  once a target printer is chosen.
- `internal/api` — HTTP handlers and routing (`Server`).
- `cmd/core-api` — the runnable entrypoint.

## Data model notes

- A `Tenant`'s ID doubles as its Zitadel organization ID (one tenant = one
  Zitadel org), so provisioning a vendor user needs no separate mapping.
- A `Transaction` snapshots the product's name at creation time
  (`ProductName`), so a receipt stays accurate even if the product is later
  renamed or deleted.
- Creating a transaction (`POST /transactions`) appends it to the caller's
  currently open draft receipt in the same action — there's no separate
  "add to receipt" step, matching the app's "verify on screen, then tap to
  lock in" flow. A draft receipt's lines can be removed
  (`DELETE /receipts/current/lines/{transactionId}`) without deleting the
  underlying transaction, which stays as an audit trail.
- Finalizing (`POST /receipts/current/finalize`) requires at least one line,
  allocates a sequential per-tenant receipt number, and returns the
  rendered text/HTML alongside the receipt so the client can display/print
  it without a second call. `POST /receipts/{id}/email` only works on an
  already-finalized receipt.

## HTTP API summary

| Method & path | Access | Purpose |
|---|---|---|
| `POST /users` | admin | Create a vendor user (Zitadel identity + local record) |
| `GET /users` | admin | List the tenant's users |
| `DELETE /users/{id}` | admin | Delete a vendor user |
| `GET /products` | any | List the tenant's products |
| `POST /products` | admin | Add a product |
| `PUT /products/{id}` | admin | Update a product |
| `DELETE /products/{id}` | admin | Delete a product |
| `POST /transactions` | any | Record a scale-approved transaction; appends to the open draft receipt |
| `GET /receipts/current` | any | Get (or create) the caller's open draft receipt, with lines resolved |
| `DELETE /receipts/current/lines/{transactionId}` | any | Remove a line from the draft receipt |
| `POST /receipts/current/finalize` | any | Lock the draft receipt; returns it with rendered text/HTML |
| `POST /receipts/{id}/email` | any | Email a finalized receipt |

"any" means any authenticated user in the tenant (vendor or admin); all
operations are scoped to the caller's own tenant, and a target belonging to
another tenant is reported as 404 rather than 403 to avoid leaking
cross-tenant existence.

## Configuration (environment variables)

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | Postgres connection string |
| `LISTEN_ADDR` | HTTP listen address (default `:8081`) |
| `ZITADEL_ISSUER_URL` | Zitadel instance issuer URL, for OIDC discovery |
| `ZITADEL_AUDIENCE` | Expected token audience (API/client ID) |
| `ZITADEL_BASE_URL` | Zitadel instance base URL, for the admin API |
| `ZITADEL_SERVICE_TOKEN` | Bearer token for a Zitadel service account with user-management permissions |
| `SMTP_ADDR` | SMTP server `host:port` for sending receipts |
| `SMTP_FROM` | From address for receipt emails |

## Running

```
DATABASE_URL=... ZITADEL_ISSUER_URL=... ZITADEL_AUDIENCE=... \
ZITADEL_BASE_URL=... ZITADEL_SERVICE_TOKEN=... SMTP_ADDR=... SMTP_FROM=... \
  go run ./cmd/core-api
```

## Testing

```
go test ./...
```

Unit tests use the in-memory store, a fake token verifier, and a fake
Zitadel admin client/email sender — no live Zitadel, Postgres, or SMTP
server needed. The `internal/storage/postgres` package additionally has
real integration tests, skipped unless `DATABASE_URL` is set (they were run
and passed against a local Postgres 15 instance while building this).
`internal/receipt`'s email test runs a genuine minimal SMTP server over a
real TCP socket, so `SMTPSender` is exercised against actual wire protocol
behavior, not a mocked interface.

**Unverified against a live Zitadel instance:** `ZitadelAdminClient`'s
request/response shapes are grounded in Zitadel's own API docs (linked in
`internal/auth/admin_client.go`) since no live instance or credentials were
available while building this — check it against a real instance before
relying on it in production.
