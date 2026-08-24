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

  static const zitadelRedirectUrl = String.fromEnvironment(
    'ZITADEL_REDIRECT_URL',
    defaultValue: 'com.scaleapp.scale_app:/callback',
  );
}
