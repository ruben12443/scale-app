# scale-app

A point-of-sale system for market/direct-marketing vendors: a vendor's
phone talks to a physical price-computing scale to weigh and price goods,
builds up a receipt across a customer's purchases, and finalizes it as an
emailed or printed receipt.

Legal-metrology certification is avoided entirely by only ever integrating
with already-certified price-computing scales (e.g. Bizerba) via their own
POS integration protocols (Dialog 02/04, and later RIK) — the scale itself
computes the price and displays it to the customer; this system never
performs or certifies any weighing/pricing calculation of its own.

## Layout (monorepo)

- [`backend/`](backend/README.md) — Go services. Currently:
  [`scale-gateway`](backend/services/scale-gateway/README.md), which talks
  to the scale(s) at one location and exposes an HTTP API for the mobile
  app.
- [`mobile/`](mobile/README.md) — Flutter vendor app (not yet built).
- `.devcontainer/` — container definition for running Claude Code against
  this workspace (see root `CLAUDE.md`).
- `.github/workflows/` — CI/CD. `ci.yml` builds/vets/tests the Go backend on
  every push and pull request. `release-tag.yml` tags every push that lands
  on `master` with the next patch version (e.g. `v0.1.0` -> `v0.1.1`); in
  the intended git-flow that's a `develop` -> `master` merge.

## Branching

Development happens on `develop`; `master` tracks released state and is
updated by merging `develop` in, which triggers an automatic version tag.

## Status

Early build-out. The scale-gateway service (protocol codec, driver
abstraction, HTTP API, unit tests) is in place; tenant/user management,
product/price catalog, transaction storage, receipts, authentication
(Zitadel), payments (Stripe), the mobile app, and CI/CD are still to come.
