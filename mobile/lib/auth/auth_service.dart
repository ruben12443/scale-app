import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:oidc/oidc.dart';
import 'package:oidc_default_store/oidc_default_store.dart';

/// Zitadel OIDC configuration. There's one login flow for every user —
/// admin vs. vendor is a role on the resulting core-api user record (see
/// GET /me), not a separate authentication path.
class AuthConfig {
  final String issuer;
  final String clientId;
  final Uri redirectUri;
  final Uri? postLogoutRedirectUri;
  final List<String> scopes;

  const AuthConfig({
    required this.issuer,
    required this.clientId,
    required this.redirectUri,
    this.postLogoutRedirectUri,
    this.scopes = const ['openid', 'profile', 'email', 'offline_access'],
  });
}

/// Wraps an [OidcUserManager] (see the `oidc` package, an OpenID-certified
/// relaying party that — unlike the flutter_appauth package this replaced —
/// actually supports web as well as Android/iOS/macOS/desktop). The manager
/// owns everything AuthState used to do by hand: opening the system browser
/// (native) or navigating the page (web) for the Authorization Code + PKCE
/// flow, persisting the session in secure storage, and keeping the access
/// token fresh (automatic background refresh) — AuthState just mirrors
/// [userChanges] rather than managing any of that itself.
///
/// Not unit-testable beyond AuthState's use of it (see the mobile README)
/// without a live IdP and a real browser redirect.
class AuthService {
  final OidcUserManager manager;

  AuthService(AuthConfig config, {OidcUserManager? manager})
    : manager = manager ?? _buildManager(config);

  static OidcUserManager _buildManager(AuthConfig config) {
    final manager = OidcUserManager.lazy(
      discoveryDocumentUri: OidcUtils.getOpenIdConfigWellKnownUri(
        Uri.parse(config.issuer),
      ),
      // Zitadel's native/SPA clients are public (no client secret) — PKCE
      // is what secures the code exchange instead.
      clientCredentials: OidcClientAuthentication.none(
        clientId: config.clientId,
      ),
      store: OidcDefaultStore(),
      settings: OidcUserManagerSettings(
        scope: config.scopes,
        redirectUri: config.redirectUri,
        postLogoutRedirectUri: config.postLogoutRedirectUri,
        // Keeps a session usable (against the cached token) through a brief
        // network drop instead of forcing a fresh login the moment a
        // background refresh fails to reach Zitadel.
        supportOfflineAuth: true,
      ),
    );
    final store = manager.store;
    if (store is OidcDefaultStore) {
      store.secureStorage = const FlutterSecureStorage();
    }
    return manager;
  }

  /// Restores a persisted session (if any) from secure storage. Must be
  /// called once before anything else on [manager].
  Future<void> init() => manager.init();

  /// Emits the current user (or null when logged out) on every change —
  /// including ones AuthState didn't itself trigger, like a background
  /// refresh failing outright.
  Stream<OidcUser?> get userChanges => manager.userChanges();

  OidcUser? get currentUser => manager.currentUser;

  Future<OidcUser?> login() => manager.loginAuthorizationCodeFlow();

  /// Returns an access token guaranteed fresh for at least a short margin,
  /// refreshing first if needed — the manager coalesces concurrent callers
  /// into a single refresh-token exchange. Returns null if not logged in.
  Future<String?> ensureFreshAccessToken() => manager.getAccessToken();

  Future<void> logout() => manager.logout();
}
