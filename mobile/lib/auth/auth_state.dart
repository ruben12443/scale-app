import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:oidc/oidc.dart';

import '../models/user.dart';
import 'auth_service.dart';

/// App-wide login state: the current Rauthy session (via [AuthService]/
/// `OidcUserManager`, which owns persistence and refresh) and the
/// corresponding core-api user record (tenant/role/display name, from
/// GET /me).
class AuthState extends ChangeNotifier {
  final AuthService _authService;
  StreamSubscription<OidcUser?>? _sub;

  OidcUser? _oidcUser;
  AppUser? currentUser;

  AuthState(this._authService) {
    _sub = _authService.userChanges.listen((user) {
      _oidcUser = user;
      if (user == null) currentUser = null;
      notifyListeners();
    });
  }

  bool get isLoggedIn => _oidcUser != null && currentUser != null;
  String? get accessToken => _oidcUser?.token.accessToken;

  /// Attempts to restore a session from secure storage (e.g. on app
  /// launch). Returns true if a session was restored; the caller still
  /// needs to fetch the user profile (see main.dart) to complete login.
  Future<bool> tryRestoreSession() async {
    await _authService.init();
    _oidcUser = _authService.currentUser;
    if (_oidcUser != null) notifyListeners();
    return _oidcUser != null;
  }

  Future<void> login() async {
    await _authService.login();
    _oidcUser = _authService.currentUser;
    notifyListeners();
  }

  /// Ensures the access token isn't about to expire, refreshing it first if
  /// needed. Call before making an API request.
  Future<String> ensureFreshAccessToken() async {
    final token = await _authService.ensureFreshAccessToken();
    if (token == null) {
      throw StateError('not logged in');
    }
    return token;
  }

  void setCurrentUser(AppUser user) {
    currentUser = user;
    notifyListeners();
  }

  Future<void> logout() async {
    await _authService.logout();
    _oidcUser = null;
    currentUser = null;
    notifyListeners();
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}
