# scale-app

A point-of-sale system for market/direct-marketing vendors: a vendor's
phone talks to a physical price-computing scale to weigh and price goods,
builds up a receipt across a customer's purchases, and finalizes it as an
emailed or printed receipt, with card-present payment via the phone acting
as a Stripe Terminal.

Legal-metrology certification is avoided entirely by only ever integrating
with already-certified price-computing scales (e.g. Bizerba) via their own
POS integration protocols (Dialog 02/04, and later RIK) — the scale itself
computes the price and displays it to the customer; this system never
performs or certifies any weighing/pricing calculation of its own.

## Layout (monorepo)

- [`backend/`](backend/README.md) — Go services:
  - [`scale-gateway`](backend/services/scale-gateway/README.md) — talks to
    the scale(s) at one location, exposes an HTTP API for the mobile app.
  - [`core-api`](backend/services/core-api/README.md) — the cloud backend:
    tenants/users, product/price catalog, transactions, draft/finalized
    receipts, Zitadel auth, Stripe payments.
- [`mobile/`](mobile/README.md) — Flutter vendor app (not yet built).
- `.devcontainer/` — container definition for running Claude Code against
  this workspace (see root `CLAUDE.md`).
- `.github/workflows/` — CI/CD. `ci.yml` builds/vets/tests the Go backend
  (with a real Postgres service, so core-api's storage integration tests
  run for real, not just skip) on every push and pull request.
  `release-tag.yml` tags every push that lands on `master` with the next
  patch version (e.g. `v0.1.0` -> `v0.1.1`); in the intended git-flow
  that's a `develop` -> `master` merge.
- `docker-compose.yml` — local dev stack: Postgres (one instance for
  core-api, a separate one for Zitadel), Zitadel, Mailpit (a local SMTP
  catcher with a web UI so receipt emails are visible during dev without a
  real mail account), scale-gateway, and core-api.

## Local development

`.env` is gitignored and not checked into the repo — create it yourself
(see `docker-compose.yml` for the variables it reads: `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB`, `ZITADEL_DB_PASSWORD`,
`ZITADEL_MASTERKEY`, `ZITADEL_AUDIENCE`, `ZITADEL_SERVICE_TOKEN`,
`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `CURRENCY`), then:

```
docker compose up --build
```

**Not automatable in the compose file itself:** on first run, Zitadel needs
manual one-time setup through its console (http://localhost:8080) — create
a project and an API application (this becomes `ZITADEL_AUDIENCE`), then a
service user with a personal access token that has user-management
permissions (this becomes `ZITADEL_SERVICE_TOKEN`). Put both in `.env` and
restart `core-api`. Stripe test-mode keys come from
https://dashboard.stripe.com/test/apikeys; for local webhook testing, the
Stripe CLI's `stripe listen --forward-to localhost:8081/webhooks/stripe`
prints a webhook secret to use as `STRIPE_WEBHOOK_SECRET`.

**Unexecuted:** this compose file hasn't been run end-to-end in the
environment this was built in (no Docker available there) — it's built
from the services' own verified configuration and Zitadel's own documented
compose pattern, but treat it as unverified until run for real. Same
caveat applies to the Zitadel and Stripe API integrations themselves; see
`backend/services/core-api/README.md` for specifics.

## Branching

Development happens on `develop`; `master` tracks released state and is
updated by merging `develop` in, which triggers an automatic version tag.

## Status

Early build-out. Both backend services (scale-gateway and core-api) are in
place with unit and integration test coverage. Still to come: the Flutter
mobile app, and running/validating the full stack (docker-compose, live
Zitadel, live Stripe) end-to-end.
