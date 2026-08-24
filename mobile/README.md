# mobile

Flutter app for scale-app's vendor-facing mobile client.

## Layout

- `lib/models/` — plain Dart types mirroring the backends' JSON shapes
  exactly (field names, nesting) — `AppUser`, `Product`, `ScaleTransaction`,
  `Receipt`, `ScaleStatus`, `ScaleWeighResult`.
- `lib/api/` — `CoreApiClient` (core-api's HTTP surface) and
  `ScaleGatewayClient` (the local scale-gateway's HTTP surface), both
  constructed with an injectable `http.Client` for testing.
- `lib/auth/` — `AuthService` wraps `flutter_appauth`'s OIDC Authorization
  Code + PKCE flow against Zitadel; `AuthState` (a `ChangeNotifier`) holds
  the session, persists the refresh token in secure storage, and refreshes
  the access token on demand (`ensureFreshAccessToken`) rather than the API
  client racing a plain in-memory value against expiry.
- `lib/state/receipt_state.dart` — the caller's current draft receipt,
  observable so any screen reflects it without re-fetching.
- `lib/screens/` — `LoginScreen`, `HomeShell` (bottom-nav shell), 
  `ScalesScreen`, `SellScreen`, `ReceiptScreen`, `AdminUsersScreen`.

## Design decisions worth knowing

- **One login flow, not two.** The original ask was "vendor login" and
  "admin login." The backend doesn't actually have two login paths — every
  user authenticates via Zitadel the same way, and `GET /me` tells the app
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

- **The Zitadel OIDC login flow itself.** `AuthService`/`AuthState` are
  built against `flutter_appauth`'s real, verified API (checked against
  its source, not guessed), and their pure logic
  (`AuthSession.isExpired`, `AuthState`'s session/refresh-token handling)
  is unit tested with a fake `AuthService` — but the actual browser
  redirect + token exchange against a live Zitadel instance has not been
  exercised, since no live instance was available while building this.
- **Live device/emulator rendering.** No Android/iOS SDK or Chrome is
  available in the environment this was built in, so nothing has been
  visually confirmed on a device, simulator, or in a browser — only
  `flutter analyze`, `flutter test`, and a native Linux release build.
- **Stripe Terminal / Tap to Pay in the app itself.** Not yet wired up —
  core-api's `POST /payments/connection-token` and
  `POST /receipts/{id}/payment` exist and are tested server-side, but the
  mobile-side Stripe Terminal SDK integration (using the connection token
  to actually collect a card-present payment) isn't built yet.

## Configuration

Overridable at build/run time via `--dart-define` (see `lib/config.dart`
for the full list and defaults, which point at the local docker-compose
stack): `CORE_API_BASE_URL`, `SCALE_GATEWAY_BASE_URL`, `ZITADEL_ISSUER`,
`ZITADEL_CLIENT_ID`, `ZITADEL_REDIRECT_URL`.

Note `SCALE_GATEWAY_BASE_URL` is a single, build-time value — in reality
each stall has its own scale-gateway instance on its own local network, so
a real deployment would need this to be configurable per-session (e.g. a
settings screen or QR-code pairing) rather than baked in. Out of scope for
this first draft.
