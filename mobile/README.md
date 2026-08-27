# mobile

Flutter app for scale-app's vendor-facing mobile client.

## Layout

- `lib/models/` — plain Dart types mirroring the backends' JSON shapes
  exactly (field names, nesting) — `AppUser`, `Product`, `ScaleTransaction`,
  `Receipt`, `ScaleStatus`, `ScaleWeighResult`.
- `lib/api/` — `CoreApiClient` (core-api's HTTP surface) and
  `ScaleGatewayClient` (the local scale-gateway's HTTP surface), both
  constructed with an injectable `http.Client` for testing.
- `lib/auth/` — `AuthService` wraps an `OidcUserManager` (the `oidc`
  package — an OpenID-certified relying party with real Android/iOS/macOS/
  web/desktop support, unlike its predecessor `flutter_appauth`, which had
  no web implementation at all) for the Authorization Code + PKCE flow
  against Rauthy. The manager owns session persistence and background
  token refresh itself; `AuthState` (a `ChangeNotifier`) just mirrors its
  `userChanges` stream and exposes `ensureFreshAccessToken()` for the API
  client to call before each request.
- `lib/state/receipt_state.dart` — the caller's current draft receipt,
  observable so any screen reflects it without re-fetching.
- `lib/screens/` — `LoginScreen`, `HomeShell` (bottom-nav shell), 
  `ScalesScreen`, `SellScreen`, `ReceiptScreen`, `AdminUsersScreen`.

## Design decisions worth knowing

- **One login flow, not two.** The original ask was "vendor login" and
  "admin login." The backend doesn't actually have two login paths — every
  user authenticates via Rauthy the same way, and `GET /me` tells the app
  whether that account is `admin` or `vendor`. `HomeShell` shows the
  Vendors (admin user management) tab only when the fetched role is admin;
  the backend enforces this independently via `RequireRole`, so this is
  just UI convenience, not the actual access control.
- **The scale, not this app, computes a weighed price.** `SellScreen`'s
  "verify" step displays what `scale-gateway` already read back from the
  scale — weight, unit price, total — and only ever sends that same data
  on to core-api. It never recomputes anything. A per-piece product skips
  the scale entirely: the app picks a quantity and multiplies by price
  itself, which is fine — no physical measurement, no metrology concern.
- **Removing a receipt line doesn't delete the transaction.** Matches the
  backend: `DELETE /receipts/current/lines/{id}` unlinks a line from the
  draft; the underlying transaction record (what the scale actually
  measured, or what was counted) stays as an audit trail.
- **Finalizing isn't final — sending is.** `ReceiptScreen` lets a
  finalized receipt be reopened back into a draft (e.g. to fix a
  mis-scanned line); only emailing it locks the receipt for good, showing
  a "locked" notice instead of any further actions.
- Product management (create/edit) isn't in this app — the original spec
  only calls for *picking* a product with its existing price, not managing
  the catalog from the phone.
- **Only one vendor can be on a scale at a time.** Tapping a scale on
  `ScalesScreen` claims it (`POST /scales/{id}/claim` — see
  `backend/services/scale-gateway/README.md`); a scale another vendor
  already holds shows who's using it and isn't tappable. `SellScreen` holds
  that claim for as long as it's open, renewing it every 5s
  (`sellScreenClaimRenewalInterval`) — well inside the gateway's own 20s
  claim TTL — and releases it as soon as the vendor is done or has stopped
  paying attention:
  - adding a line to the receipt jumps straight back to `ScalesScreen`
    rather than staying on the sell flow, since that's normally the start
    of the next sale;
  - `sellScreenInactivityTimeout` (7s) with no touch on the screen also
    bounces back to `ScalesScreen`, so a vendor who wanders off doesn't
    block the scale for everyone else;
  - the app backgrounding — including the phone's screen locking — releases
    the claim immediately (via `WidgetsBindingObserver`) rather than waiting
    for it to expire, and resuming always lands back on `ScalesScreen`
    rather than resuming mid-sale.
  
  The claim is a courtesy reservation identified by the vendor's core-api
  user id, not an auth boundary — scale-gateway trusts callers on the local
  network, same as everything else it does.

## Verified vs. not

Run from `mobile/`:

```
flutter analyze   # 0 issues
flutter test      # all unit/state tests pass — no device or emulator needed
flutter build linux --release   # compiles the full app incl. all plugins
```

All of the above were run and passed while building this. What's **not**
verified, consistent with the other integrations flagged elsewhere in this
repo:

- **The Rauthy OIDC login flow itself.** `AuthService`/`AuthState` are
  built against the `oidc` package's real, verified API (checked against
  its source, not guessed — including copying its `web/redirect.html`
  verbatim, as its own docs require), and `AuthState`'s session-mirroring
  logic is unit tested against a fake `AuthService` (constructing real,
  if unverified, `OidcUser`/`OidcToken` objects rather than a bespoke
  session type) — but the actual browser redirect + token exchange against
  a live Rauthy instance has not been exercised, since no live instance
  or `RAUTHY_CLIENT_ID` was available while building this. The token
  expiry/refresh logic itself is no longer ours to unit-test at all — it's
  now inside the `oidc` package (OpenID-certified, with its own test
  suite), which is arguably more trustworthy than the hand-rolled version
  it replaced, but it does mean this repo has one less unit it can test
  directly.
- **Live device/emulator rendering.** No Android/iOS SDK or Chrome is
  available in the environment this was built in, so nothing has been
  visually confirmed on a device, simulator, or in a browser — only
  `flutter analyze`, `flutter test`, and a native Linux release build.
- **Stripe Terminal / Tap to Pay in the app itself.** Not yet wired up —
  core-api's `POST /payments/connection-token` and
  `POST /receipts/{id}/payment` exist and are tested server-side, but the
  mobile-side Stripe Terminal SDK integration (using the connection token
  to actually collect a card-present payment) isn't built yet.
- **`SellScreen`'s claim renewal, inactivity timeout, and app-lifecycle
  handling.** The claim/release HTTP calls they drive are unit tested at
  the `ScaleGatewayClient` level, but the `Timer`-, `AnimationController`-,
  and `WidgetsBindingObserver`-based logic in `SellScreen` itself (renew
  every 5s, bounce back when the on-screen countdown bar empties at 7s,
  release on backgrounding/screen lock) has no widget test — this repo
  doesn't have widget tests for any screen yet, and adding the first one
  was out of scope here. `flutter analyze` and `flutter test` pass; a
  native Linux release build wasn't re-verified in the environment that
  added this, since it lacked `clang`/`cmake`/`ninja`.
- **Android's native redirect wiring.** `android/app/build.gradle.kts` now
  sets `manifestPlaceholders["oidcRedirectScheme"]`, which `oidc_android`
  merges into the app's manifest to register the redirect-catching
  activity — this was never wired up before (there was no manifest
  placeholder for `flutter_appauth` either, a pre-existing gap this closed
  in passing rather than one this change introduced). iOS needs no
  equivalent Info.plist entry: `ASWebAuthenticationSession` matches the
  callback scheme at runtime. Neither has been exercised on a real device.

## Configuration

Overridable at build/run time via `--dart-define` (see `lib/config.dart`
for the full list and defaults, which point at the local docker-compose
stack): `CORE_API_BASE_URL`, `SCALE_GATEWAY_BASE_URL`, `RAUTHY_ISSUER`,
`RAUTHY_CLIENT_ID`, `RAUTHY_REDIRECT_URL` (native only — the custom-scheme
callback), `RAUTHY_WEB_REDIRECT_PATH` (web only — a path resolved against
wherever the app is actually served from; defaults to `redirect.html` and
rarely needs overriding).

Note `SCALE_GATEWAY_BASE_URL` is a single, build-time value — in reality
each stall has its own scale-gateway instance on its own local network, so
a real deployment would need this to be configurable per-session (e.g. a
settings screen or QR-code pairing) rather than baked in. Out of scope for
this first draft.

## Progressive Web App

`web/` (added via `flutter create --platforms=web .`) makes this a
standard installable PWA: `web/manifest.json` (name "Stallhand", the
market-green/paper theme and background colors, `display: standalone`)
plus a maskable + regular icon set at `web/icons/` and `web/favicon.png`
(currently a simple generated scale-plate glyph on the brand green —
**a placeholder, not a designed logo**; swap these before this goes in
front of real vendors). `flutter build web` also emits
`flutter_service_worker.js` automatically, which Flutter's own
`flutter_bootstrap.js` loader registers — no extra wiring needed for that
part.

**How a vendor actually gets the app**: they visit the page in their
phone's browser and use the browser's own "Add to Home Screen" / install
prompt — there's no app store listing and nothing to sideload. Locally,
`docker-compose.yml`'s `mobile-web` service builds and serves exactly this
(`mobile/Dockerfile`, multi-stage: `flutter build web` then plain nginx)
on port 8083, using `RAUTHY_DOMAIN` the same way the `rauthy` service
already does — whatever host (a LAN IP for a phone, `localhost` for a
desktop browser) the vendor's device will actually reach this stack on.

Login itself works the same way on web as everywhere else — see
`lib/auth/`'s notes above on `flutter_appauth` (which had no web support at
all) having been replaced by the `oidc` package before Rauthy even entered
the picture. `web/redirect.html` (copied verbatim from that package, per
its own instructions — re-copy it if the package is ever upgraded) is the
page the Rauthy redirect actually lands on. One thing this local setup
does **not** yet cover:

- **Production hosting.** `mobile-web` is a local/dev convenience — actual
  distribution to vendors needs real HTTPS and a real domain (browsers
  gate PWA installability on HTTPS outside of `localhost`), which is
  infrastructure this repo doesn't set up.
