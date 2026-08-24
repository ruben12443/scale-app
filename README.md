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
`POSTGRES_PASSWORD`, `POSTGRES_DB`, `ZITADEL_DOMAIN`, `ZITADEL_DB_PASSWORD`,
`ZITADEL_MASTERKEY`, `ZITADEL_AUDIENCE`, `ZITADEL_SERVICE_TOKEN`,
`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `CURRENCY`), then:

```
docker compose up --build
```

`ZITADEL_DOMAIN` is the host the mobile app/browser will reach this stack
on: `localhost` for desktop testing, or this machine's LAN IP (e.g.
`192.168.1.42`) to test from a phone on the same network — see "Testing on
a phone" below. It's baked into Zitadel's first-run instance data, so
changing it requires `docker compose down -v && docker compose up --build`
to take effect; no other file needs editing.

**Not automatable in the compose file itself:** on first run, Zitadel needs
manual one-time setup through its console (`http://<ZITADEL_DOMAIN>:8080/ui/console` —
logging in redirects through the separate login UI at
`<ZITADEL_DOMAIN>:3000`, see below) — create a project and an API
application (this becomes `ZITADEL_AUDIENCE`), then a service user with a
personal access token that has user-management permissions (this becomes
`ZITADEL_SERVICE_TOKEN`). Put both in `.env` and restart `core-api`. Stripe
test-mode keys come from https://dashboard.stripe.com/test/apikeys; for
local webhook testing, the Stripe CLI's
`stripe listen --forward-to localhost:8081/webhooks/stripe` prints a
webhook secret to use as `STRIPE_WEBHOOK_SECRET`.

Zitadel v4 splits its login UI into a separate service (`zitadel-login`,
port 3000) from the main API (`zitadel`, port 8080) — the official
reference deployment unifies both under one domain via Traefik, which this
compose file skips for local-dev simplicity; both are just published on
their own ports instead. Visiting `/ui/console` on 8080 will redirect
through 3000 for the actual login step, which is expected.

**Unexecuted:** this compose file hasn't been run end-to-end in the
environment this was built in (no Docker available there) — it's built
from the services' own verified configuration and Zitadel's own documented
compose pattern, but treat it as unverified until run for real. Same
caveat applies to the Zitadel and Stripe API integrations themselves; see
`backend/services/core-api/README.md` for specifics.

**No physical scale?** `scale-gateway` needs a real scale (or a
serial-to-Ethernet adapter in front of one) to do anything useful.
`backend/services/scale-gateway/cmd/fake-scale` simulates one over a real
TCP listener — see that service's README for how to point scale-gateway at
it, which lets you exercise the entire flow (mobile app → scale-gateway →
core-api → receipt) without hardware.

## Testing on a phone

By default the stack (and the mobile app's own defaults, in
`mobile/lib/config.dart`) targets `localhost`, which only works from a
browser or emulator on the same machine — a physical phone can't resolve
`localhost` to your computer. To test on a phone on the same Wi-Fi network:

1. Find this machine's LAN IP (e.g. `192.168.1.42`).
2. Set `ZITADEL_DOMAIN=192.168.1.42` in `.env`, then
   `docker compose down -v && docker compose up --build` (a fresh Zitadel
   init is required — the domain is baked into its first-run instance
   data).
3. Run the app pointed at the same IP for every backend it talks to:
   ```
   flutter run \
     --dart-define=CORE_API_BASE_URL=http://192.168.1.42:8081 \
     --dart-define=SCALE_GATEWAY_BASE_URL=http://192.168.1.42:8082 \
     --dart-define=ZITADEL_ISSUER=http://192.168.1.42:8080 \
     --dart-define=ZITADEL_CLIENT_ID=<your API application's client ID>
   ```

All four values must agree with each other and with `ZITADEL_DOMAIN` —
mixing `localhost` and a LAN IP across them breaks OIDC discovery (the
issuer Zitadel advertises must match the URL the phone actually reached it
at) and CORS/redirect matching. Switch back to `localhost` the same way
when done.

## Branching

Development happens on `develop`; `master` tracks released state and is
updated by merging `develop` in, which triggers an automatic version tag.

## Status

Early build-out. Both backend services (scale-gateway and core-api) are in
place with unit and integration test coverage. Still to come: the Flutter
mobile app, and running/validating the full stack (docker-compose, live
Zitadel, live Stripe) end-to-end.
