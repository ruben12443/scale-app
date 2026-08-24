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
- `internal/payment` — Stripe integration for phone-as-terminal (Stripe
  Terminal / Tap to Pay) payments: connection tokens, PaymentIntents, and
  status lookups, behind a `Processor` interface with a `FakeProcessor` for
  tests.
- `internal/api` — HTTP handlers and routing (`Server`).
- `cmd/core-api` — the runnable entrypoint.

## Data model notes

- A `Tenant`'s ID doubles as its Zitadel organization ID (one tenant = one
  Zitadel org), so provisioning a vendor user needs no separate mapping.
- A `Transaction` snapshots the product's name at creation time
  (`ProductName`), so a receipt stays accurate even if the product is later
  renamed or deleted.
- A `Product` is priced either `per_kg` (weighed on a certified scale) or
  `per_piece` (counted). A `Transaction` carries the same `PricingType` and
  either `WeightGrams` (per-kg, scale-computed) or `Quantity` (per-piece,
  ordinary quantity-times-price arithmetic — no scale involved, and no
  legal-metrology concern, since nothing is physically measured). Which
  fields `POST /transactions` requires is derived from the referenced
  product's own pricing type, not the caller's say-so.
- Creating a transaction (`POST /transactions`) appends it to the caller's
  currently open draft receipt in the same action — there's no separate
  "add to receipt" step, matching the app's "verify on screen, then tap to
  lock in" flow. A draft receipt's lines can be removed
  (`DELETE /receipts/current/lines/{transactionId}`) without deleting the
  underlying transaction, which stays as an audit trail.
- A receipt moves through three states: `draft` (mutable) ->
  `finalized` (locked from editing, but `POST /receipts/{id}/reopen` puts
  it back into `draft`, e.g. to fix a mis-scanned line) -> `sent` (emailed,
  or later printed — the real point of no return; a sent receipt can never
  be reopened or mutated again).
- Finalizing (`POST /receipts/current/finalize`) requires at least one line,
  allocates a sequential per-tenant receipt number, and returns the
  rendered text/HTML alongside the receipt so the client can display/print
  it without a second call. Reopening clears the number and finalized-at
  timestamp; re-finalizing allocates a fresh number. `POST /receipts/{id}/email`
  only works on a finalized (not yet sent) receipt, and marks it `sent` on
  success.

## HTTP API summary

| Method & path | Access | Purpose |
|---|---|---|
| `GET /me` | any | The caller's own user record (tenant, role, display name) — the only way a client learns this after Zitadel login |
| `POST /users` | admin | Create a vendor user (Zitadel identity + local record) |
| `GET /users` | admin | List the tenant's users |
| `DELETE /users/{id}` | admin | Delete a vendor user |
| `GET /products` | any | List the tenant's products |
| `POST /products` | admin | Add a product |
| `PUT /products/{id}` | admin | Update a product |
| `DELETE /products/{id}` | admin | Delete a product |
| `POST /transactions` | any | Record a weighed or per-piece sale line; appends to the open draft receipt |
| `GET /receipts/current` | any | Get (or create) the caller's open draft receipt, with lines resolved |
| `DELETE /receipts/current/lines/{transactionId}` | any | Remove a line from the draft receipt |
| `POST /receipts/current/finalize` | any | Lock the draft receipt; returns it with rendered text/HTML |
| `POST /receipts/{id}/reopen` | any | Put a finalized (not yet sent) receipt back into draft |
| `POST /receipts/{id}/email` | any | Email a finalized receipt; marks it sent (locked for good) |
| `POST /payments/connection-token` | any | Get a Stripe Terminal SDK connection token for the mobile app |
| `POST /receipts/{id}/payment` | any | Start a card-present charge for a finalized receipt's total |
| `POST /webhooks/stripe` | Stripe signature, not a bearer token | Payment intent status updates |

"any" means any authenticated user in the tenant (vendor or admin); all
operations are scoped to the caller's own tenant, and a target belonging to
another tenant is reported as 404 rather than 403 to avoid leaking
cross-tenant existence.

## Configuration (environment variables)

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | Postgres connection string |
| `LISTEN_ADDR` | HTTP listen address (default `:8081`) |
| `ZITADEL_ISSUER_URL` | Zitadel's externally-configured issuer (must match its `ZITADEL_EXTERNALDOMAIN`/port exactly — used for token issuer validation) |
| `ZITADEL_DISCOVERY_URL` | Address core-api actually reaches Zitadel at to fetch its OIDC discovery document/JWKS; defaults to `ZITADEL_ISSUER_URL` if unset |
| `ZITADEL_AUDIENCE` | Expected token audience (API/client ID) |
| `ZITADEL_BASE_URL` | Zitadel instance base URL, for the admin API |
| `ZITADEL_SERVICE_TOKEN` | Bearer token for a Zitadel service account with user-management permissions |
| `SMTP_ADDR` | SMTP server `host:port` for sending receipts |
| `SMTP_FROM` | From address for receipt emails |
| `STRIPE_SECRET_KEY` | Stripe secret API key |
| `STRIPE_WEBHOOK_SECRET` | Signing secret for the `/webhooks/stripe` endpoint (from the Stripe dashboard) |
| `CURRENCY` | ISO currency code for payment intents, lowercase (default `chf`) |

`ZITADEL_ISSUER_URL` and `ZITADEL_DISCOVERY_URL` are only ever different
when core-api reaches Zitadel over an internal address (e.g. the
`http://zitadel:8080` Docker Compose service name) that isn't the same
address Zitadel was told is its own public identity (e.g. `localhost` or a
LAN IP for phone testing). OIDC discovery requires the fetched document's
`issuer` field to match the URL it was fetched from, so a single shared URL
breaks the moment those two addresses differ — see
`internal/auth/verifier.go`'s doc comment for the mechanism
(`oidc.InsecureIssuerURLContext`) that makes the split work.

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

**Stripe** uses the official `stripe-go` SDK, so its request/response
shapes are correct by construction (the compiler enforces the real API
surface) — but `StripeProcessor` itself (the code that actually calls
Stripe) hasn't been exercised against a live Stripe account, only via
`FakeProcessor` in tests. The webhook handler's signature verification
*is* tested for real, using the SDK's own `webhook.GenerateTestSignedPayload`
test helper — no live account needed for that part. Note
`IgnoreAPIVersionMismatch: true` is set deliberately: Stripe sends webhook
events at whatever API version the dashboard endpoint is configured for,
which can drift from whatever `stripe-go` version this service is pinned
to, and the handler only reads stable fields (event type, payment intent
ID) so a mismatch isn't a real risk here.
