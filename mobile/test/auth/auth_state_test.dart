import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:oidc/oidc.dart';
import 'package:scale_app/auth/auth_service.dart';
import 'package:scale_app/auth/auth_state.dart';
import 'package:scale_app/models/user.dart';

/// An unverified id_token JWT good enough for [OidcUser.fromIdToken] to
/// parse: no keystore is passed, so the signature is never checked, only
/// the claims are decoded.
String _fakeIdToken(Map<String, dynamic> claims) {
  String segment(Map<String, dynamic> json) =>
      base64Url.encode(utf8.encode(jsonEncode(json))).replaceAll('=', '');
  final header = segment({'alg': 'none', 'typ': 'JWT'});
  final payload = segment({
    'sub': 'test-subject',
    'iss': 'https://zitadel.test',
    ...claims,
  });
  return '$header.$payload.';
}

Future<OidcUser> _fakeOidcUser({
  required String accessToken,
  String? refreshToken,
  Duration? expiresIn,
  DateTime? creationTime,
}) {
  return OidcUser.fromIdToken(
    token: OidcToken(
      creationTime: creationTime ?? DateTime.now().toUtc(),
      accessToken: accessToken,
      refreshToken: refreshToken,
      idToken: _fakeIdToken(const {}),
      expiresIn: expiresIn,
    ),
  );
}

class FakeAuthService extends AuthService {
  final _controller = StreamController<OidcUser?>.broadcast();
  OidcUser? _current;

  int loginCalls = 0;
  int logoutCalls = 0;
  int initCalls = 0;
  OidcUser? Function()? loginResult;
  Object? loginError;

  FakeAuthService()
    : super(
        AuthConfig(
          issuer: 'https://zitadel.test',
          clientId: 'test-client',
          redirectUri: Uri.parse('com.scaleapp.stallhand:/callback'),
        ),
      );

  /// Seeds the "already restored from storage" case for [init].
  Future<void> seedCurrentUser(OidcUser user) async {
    _current = user;
  }

  @override
  Stream<OidcUser?> get userChanges => _controller.stream;

  @override
  OidcUser? get currentUser => _current;

  @override
  Future<void> init() async {
    initCalls++;
  }

  @override
  Future<OidcUser?> login() async {
    loginCalls++;
    if (loginError != null) throw loginError!;
    _current = loginResult != null
        ? loginResult!()
        : await _fakeOidcUser(
            accessToken: 'access-1',
            refreshToken: 'refresh-1',
          );
    _controller.add(_current);
    return _current;
  }

  @override
  Future<String?> ensureFreshAccessToken() async => _current?.token.accessToken;

  @override
  Future<void> logout() async {
    logoutCalls++;
    _current = null;
    _controller.add(null);
  }
}

AppUser _testAppUser() => AppUser(
  id: 'u1',
  tenantId: 't1',
  zitadelSubjectId: 'sub-1',
  displayName: 'Jane',
  email: 'jane@example.com',
  role: 'vendor',
  createdAt: DateTime.now(),
);

void main() {
  group('AuthState.login', () {
    test('stores the session', () async {
      final fakeService = FakeAuthService();
      final state = AuthState(fakeService);

      await state.login();

      expect(fakeService.loginCalls, 1);
      expect(state.accessToken, 'access-1');
    });

    test('isLoggedIn is false until a user profile is also set', () async {
      final state = AuthState(FakeAuthService());
      await state.login();
      expect(state.isLoggedIn, isFalse);

      state.setCurrentUser(_testAppUser());
      expect(state.isLoggedIn, isTrue);
    });
  });

  group('AuthState.tryRestoreSession', () {
    test('returns false when no session is persisted', () async {
      final state = AuthState(FakeAuthService());
      expect(await state.tryRestoreSession(), isFalse);
    });

    test('restores a persisted session via AuthService.init', () async {
      final fakeService = FakeAuthService();
      await fakeService.seedCurrentUser(
        await _fakeOidcUser(accessToken: 'restored-access'),
      );
      final state = AuthState(fakeService);

      final restored = await state.tryRestoreSession();

      expect(restored, isTrue);
      expect(fakeService.initCalls, 1);
      expect(state.accessToken, 'restored-access');
    });
  });

  group('AuthState.ensureFreshAccessToken', () {
    test('returns the token AuthService reports', () async {
      final fakeService = FakeAuthService();
      final state = AuthState(fakeService);
      await state.login();

      final token = await state.ensureFreshAccessToken();

      expect(token, 'access-1');
    });

    test('throws if not logged in', () async {
      final state = AuthState(FakeAuthService());
      expect(() => state.ensureFreshAccessToken(), throwsStateError);
    });
  });

  group('AuthState userChanges stream', () {
    test(
      'an out-of-band logout (e.g. background refresh failure) is reflected',
      () async {
        final fakeService = FakeAuthService();
        final state = AuthState(fakeService);
        await state.login();
        state.setCurrentUser(_testAppUser());
        expect(state.isLoggedIn, isTrue);

        await fakeService.logout();
        // The stream listener updates state asynchronously.
        await Future<void>.delayed(Duration.zero);

        expect(state.isLoggedIn, isFalse);
        expect(state.accessToken, isNull);
      },
    );
  });

  group('AuthState.logout', () {
    test('clears the session and user', () async {
      final fakeService = FakeAuthService();
      final state = AuthState(fakeService);
      await state.login();
      state.setCurrentUser(_testAppUser());

      await state.logout();

      expect(fakeService.logoutCalls, 1);
      expect(state.isLoggedIn, isFalse);
      expect(state.accessToken, isNull);
      expect(state.currentUser, isNull);
    });
  });
}
