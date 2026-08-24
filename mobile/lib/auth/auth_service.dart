import 'package:flutter_appauth/flutter_appauth.dart';

/// Zitadel OIDC configuration. There's one login flow for every user —
/// admin vs. vendor is a role on the resulting core-api user record (see
/// GET /me), not a separate authentication path.
class AuthConfig {
  final String issuer;
  final String clientId;
  final String redirectUrl;
  final List<String> scopes;

  const AuthConfig({
    required this.issuer,
    required this.clientId,
    required this.redirectUrl,
    this.scopes = const ['openid', 'profile', 'email', 'offline_access'],
  });
}

/// The tokens from a successful login/refresh.
class AuthSession {
  final String accessToken;
  final String? refreshToken;
  final DateTime? accessTokenExpiration;

  const AuthSession({
    required this.accessToken,
    this.refreshToken,
    this.accessTokenExpiration,
  });

  /// True once we're within [skew] of expiry (or past it), so callers
  /// refresh a little early rather than racing a request against expiry.
  bool isExpired({Duration skew = const Duration(seconds: 30)}) {
    final expiry = accessTokenExpiration;
    if (expiry == null) return false;
    return DateTime.now().isAfter(expiry.subtract(skew));
  }
}

/// Wraps flutter_appauth's OIDC Authorization Code + PKCE flow against
/// Zitadel. Not unit-testable beyond its pure logic (AuthSession.isExpired)
/// without a live IdP and a real browser redirect — see the mobile README.
class AuthService {
  final AuthConfig config;
  final FlutterAppAuth _appAuth;

  AuthService(this.config, {FlutterAppAuth? appAuth})
    : _appAuth = appAuth ?? const FlutterAppAuth();

  Future<AuthSession> login() async {
    final result = await _appAuth.authorizeAndExchangeCode(
      AuthorizationTokenRequest(
        config.clientId,
        config.redirectUrl,
        issuer: config.issuer,
        scopes: config.scopes,
      ),
    );
    final accessToken = result.accessToken;
    if (accessToken == null) {
      throw StateError('Zitadel login did not return an access token');
    }
    return AuthSession(
      accessToken: accessToken,
      refreshToken: result.refreshToken,
      accessTokenExpiration: result.accessTokenExpirationDateTime,
    );
  }

  Future<AuthSession> refresh(String refreshToken) async {
    final result = await _appAuth.token(
      TokenRequest(
        config.clientId,
        config.redirectUrl,
        issuer: config.issuer,
        refreshToken: refreshToken,
        grantType: GrantType.refreshToken,
        scopes: config.scopes,
      ),
    );
    final accessToken = result.accessToken;
    if (accessToken == null) {
      throw StateError('Zitadel token refresh did not return an access token');
    }
    return AuthSession(
      accessToken: accessToken,
      refreshToken: result.refreshToken ?? refreshToken,
      accessTokenExpiration: result.accessTokenExpirationDateTime,
    );
  }
}
