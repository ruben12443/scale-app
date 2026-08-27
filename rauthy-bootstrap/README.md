# rauthy-bootstrap

Mounted into the `rauthy` container's `bootstrap_dir` (see `docker-compose.yml`).
Rauthy reads these once, only while initializing an *empty* database on the
very first start — editing them later has no effect on an already-running
instance. To pick up a change, `docker compose down -v` (drops the `rauthy-data`
volume) then `docker compose up --build` again, the same "fresh init required"
workflow the previous Zitadel setup already needed for a `ZITADEL_DOMAIN`
change.

- `clients.json` — registers the mobile app (`Stallhand`) as a public,
  PKCE-only OIDC client (no `secret` field — see the `oidc` package
  integration in `mobile/lib/auth/`). `redirect_uris` covers the native
  custom-scheme callback (host-independent) and the web PWA's
  `redirect.html` on `mobile-web`'s default `localhost:8083`. **Testing from
  a phone on a LAN IP, or running `flutter run -d chrome` on a different
  port, needs its own redirect URI added here** — Rauthy only allows a
  wildcard at the *end* of a redirect URI, which doesn't help when the part
  that varies (the host) is at the start, so each origin you actually test
  from has to be listed explicitly.
- `api_keys.json` — an API key core-api uses to create/delete vendor users
  (see `RAUTHY_API_KEY` in `backend/services/core-api/README.md`). Its
  `exp` is deliberately far in the future (2030), not the short-lived
  window Rauthy's own docs recommend for *production* bootstrap keys —
  this key is this local dev stack's only way for core-api to reach
  Rauthy's admin API at all, not a one-time provisioning step, so it needs
  to keep working for the life of the stack. Don't carry that choice into a
  real deployment; provision a short-lived key there and rotate it through
  Rauthy's admin UI instead.

The secret in `api_keys.json` must match `${RAUTHY_API_KEY}` in `.env`
(`core-api$<secret>` — see `docker-compose.yml`'s `core-api` service).
