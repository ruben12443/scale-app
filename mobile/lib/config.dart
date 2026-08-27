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

  static const zitadelIssuer = String.fromEnvironment(
    'ZITADEL_ISSUER',
    defaultValue: 'http://localhost:8080',
  );

  static const zitadelClientId = String.fromEnvironment('ZITADEL_CLIENT_ID');

  /// Android/iOS/macOS/desktop use a fixed custom URL scheme: the OS hands
  /// the redirect straight back to this app. Deliberately not the app's
  /// `com.scaleapp.scale_app` application ID — a URI scheme's grammar
  /// (RFC 3986 §3.1) doesn't allow `_`, unlike an Android application ID.
  /// Android's half of this is registered via
  /// `manifestPlaceholders["oidcRedirectScheme"]` in
  /// android/app/build.gradle.kts, which must match this value; iOS needs
  /// no Info.plist entry (ASWebAuthenticationSession matches the scheme at
  /// runtime, not via a registered URL type).
  static const _nativeRedirectUrl = String.fromEnvironment(
    'ZITADEL_REDIRECT_URL',
    defaultValue: 'com.scaleapp.stallhand:/callback',
  );

  /// Web can't use a custom scheme — the redirect has to land on a real
  /// page served from the same origin as the app (see web/redirect.html,
  /// copied verbatim from the `oidc` package). Given as a path so it
  /// resolves against wherever the app actually got served from
  /// (`--web-hostname`/`--web-port`, a LAN IP for phone testing, or
  /// whatever host `mobile-web`'s nginx ends up on) instead of a fixed
  /// origin baked in at build time.
  static const _webRedirectPath = String.fromEnvironment(
    'ZITADEL_WEB_REDIRECT_PATH',
    defaultValue: 'redirect.html',
  );

  static Uri get zitadelRedirectUri =>
      kIsWeb ? _resolveWebUri(_webRedirectPath) : Uri.parse(_nativeRedirectUrl);

  /// Reuses the same redirect page for post-logout on web; native platforms
  /// don't currently do RP-initiated logout (see AuthService), so this is
  /// only ever read on web.
  static Uri get zitadelWebPostLogoutRedirectUri =>
      _resolveWebUri(_webRedirectPath);

  static Uri _resolveWebUri(String path) {
    final base = Uri.base;
    final directoryPath = base.path.endsWith('/')
        ? base.path
        : base.path.substring(0, base.path.lastIndexOf('/') + 1);
    return Uri.parse(base.origin).replace(path: '$directoryPath$path');
  }
}
