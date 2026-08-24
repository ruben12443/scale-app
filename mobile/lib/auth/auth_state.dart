import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../models/user.dart';
import 'auth_service.dart';

/// App-wide login state: the current Zitadel session and the corresponding
/// core-api user record (tenant/role/display name, from GET /me).
///
/// The refresh token is persisted in secure storage so a restart doesn't
/// force a fresh login; the access token itself is kept in memory only.
class AuthState extends ChangeNotifier {
  static const _refreshTokenKey = 'zitadel_refresh_token';

  final AuthService _authService;
  final FlutterSecureStorage _storage;

  AuthSession? _session;
  AppUser? currentUser;

  AuthState(this._authService, {FlutterSecureStorage? storage})
    : _storage = storage ?? const FlutterSecureStorage();

  bool get isLoggedIn => _session != null && currentUser != null;
  String? get accessToken => _session?.accessToken;

  /// Attempts to restore a session from a stored refresh token (e.g. on app
  /// launch). Returns true if a session was restored; the caller still
  /// needs to fetch the user profile (see main.dart) to complete login.
  Future<bool> tryRestoreSession() async {
    final refreshToken = await _storage.read(key: _refreshTokenKey);
    if (refreshToken == null) return false;
    try {
      _session = await _authService.refresh(refreshToken);
      await _persistRefreshToken();
      notifyListeners();
      return true;
    } catch (_) {
      // Stored token no longer works (revoked, expired past its own
      // lifetime, etc.) - fall through to a normal interactive login.
      await _storage.delete(key: _refreshTokenKey);
      return false;
    }
  }

  Future<void> login() async {
    _session = await _authService.login();
    await _persistRefreshToken();
    notifyListeners();
  }

  /// Ensures the access token isn't about to expire, refreshing it first if
  /// needed. Call before making an API request.
  Future<String> ensureFreshAccessToken() async {
    final session = _session;
    if (session == null) {
      throw StateError('not logged in');
    }
    if (!session.isExpired()) {
      return session.accessToken;
    }
    final refreshToken = session.refreshToken;
    if (refreshToken == null) {
      throw StateError(
        'access token expired and no refresh token is available',
      );
    }
    _session = await _authService.refresh(refreshToken);
    await _persistRefreshToken();
    notifyListeners();
    return _session!.accessToken;
  }

  void setCurrentUser(AppUser user) {
    currentUser = user;
    notifyListeners();
  }

  Future<void> logout() async {
    _session = null;
    currentUser = null;
    await _storage.delete(key: _refreshTokenKey);
    notifyListeners();
  }

  Future<void> _persistRefreshToken() async {
    final refreshToken = _session?.refreshToken;
    if (refreshToken != null) {
      await _storage.write(key: _refreshTokenKey, value: refreshToken);
    }
  }
}
