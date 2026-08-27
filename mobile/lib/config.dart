import 'package:flutter/foundation.dart' show kIsWeb;

/// Runtime configuration, overridable at build time, e.g.:
///   flutter run --dart-define=CORE_API_BASE_URL=https://api.example.com
///
/// Defaults point at the local docker-compose stack (see the root
/// docker-compose.yml).
class AppConfig {
  static const coreApiBaseUrl = String.fromEnvironment(
    'CORE_API_BASE_URL',
    defaultValue: 'http://localhost:8081',
  );

  /// The scale-gateway instance for the stall this device is at. In
  /// practice this varies per location, so a real deployment would let the
  /// vendor configure it rather than baking in one value — out of scope
  /// for this first draft.
  static const scaleGatewayBaseUrl = String.fromEnvironment(
    'SCALE_GATEWAY_BASE_URL',
    defaultValue: 'http://localhost:8082',
  );

  /// Rauthy's issuer, including its `/auth/v1` path prefix (e.g.
  /// `http://localhost:8080/auth/v1`) — replaced Zitadel; see
  /// backend/services/core-api/README.md for the equivalent server-side
  /// config.
  static const rauthyIssuer = String.fromEnvironment(
    'RAUTHY_ISSUER',
    defaultValue: 'http://localhost:8080/auth/v1',
  );

  /// Must match the static client registered in
  /// rauthy-bootstrap/clients.json's `"id"` field.
  static const rauthyClientId = String.fromEnvironment('RAUTHY_CLIENT_ID');

  /// Android/iOS/macOS/desktop use a fixed custom URL scheme: the OS hands
  /// the redirect straight back to this app. Deliberately not the app's
  /// `com.scaleapp.scale_app` application ID — a URI scheme's grammar
  /// (RFC 3986 §3.1) doesn't allow `_`, unlike an Android application ID.
  /// Android's half of this is registered via
  /// `manifestPlaceholders["oidcRedirectScheme"]` in
  /// android/app/build.gradle.kts, which must match this value; iOS needs
  /// no Info.plist entry (ASWebAuthenticationSession matches the scheme at
  /// runtime, not via a registered URL type). Either way, this exact value
  /// must also be listed in rauthy-bootstrap/clients.json's redirect_uris.
  static const _nativeRedirectUrl = String.fromEnvironment(
    'RAUTHY_REDIRECT_URL',
    defaultValue: 'com.scaleapp.stallhand:/callback',
  );

  /// Web can't use a custom scheme — the redirect has to land on a real
  /// page served from the same origin as the app (see web/redirect.html,
  /// copied verbatim from the `oidc` package). Given as a path so it
  /// resolves against wherever the app actually got served from
  /// (`--web-hostname`/`--web-port`, a LAN IP for phone testing, or
  /// whatever host `mobile-web`'s nginx ends up on) instead of a fixed
  /// origin baked in at build time. The resolved origin must also be listed
  /// in rauthy-bootstrap/clients.json's redirect_uris.
  static const _webRedirectPath = String.fromEnvironment(
    'RAUTHY_WEB_REDIRECT_PATH',
    defaultValue: 'redirect.html',
  );

  static Uri get rauthyRedirectUri =>
      kIsWeb ? _resolveWebUri(_webRedirectPath) : Uri.parse(_nativeRedirectUrl);

  /// Reuses the same redirect page for post-logout on web; native platforms
  /// don't currently do RP-initiated logout (see AuthService), so this is
  /// only ever read on web.
  static Uri get rauthyWebPostLogoutRedirectUri =>
      _resolveWebUri(_webRedirectPath);

  static Uri _resolveWebUri(String path) {
    final base = Uri.base;
    final directoryPath = base.path.endsWith('/')
        ? base.path
        : base.path.substring(0, base.path.lastIndexOf('/') + 1);
    return Uri.parse(base.origin).replace(path: '$directoryPath$path');
  }
}
