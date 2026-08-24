import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_secure_storage/test/test_flutter_secure_storage_platform.dart';
import 'package:flutter_secure_storage_platform_interface/flutter_secure_storage_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:scale_app/auth/auth_service.dart';
import 'package:scale_app/auth/auth_state.dart';
import 'package:scale_app/models/user.dart';

class FakeAuthService extends AuthService {
  AuthSession loginResult = const AuthSession(
    accessToken: 'access-1',
    refreshToken: 'refresh-1',
  );
  int loginCalls = 0;
  int refreshCalls = 0;
  String? lastRefreshTokenUsed;
  Object? refreshError;

  FakeAuthService()
    : super(
        const AuthConfig(
          issuer: 'https://zitadel.test',
          clientId: 'test-client',
          redirectUrl: 'com.scaleapp.scale_app:/callback',
        ),
      );

  @override
  Future<AuthSession> login() async {
    loginCalls++;
    return loginResult;
  }

  @override
  Future<AuthSession> refresh(String refreshToken) async {
    refreshCalls++;
    lastRefreshTokenUsed = refreshToken;
    if (refreshError != null) throw refreshError!;
    return AuthSession(accessToken: 'access-refreshed', refreshToken: refreshToken);
  }
}

void main() {
  late Map<String, String> storageData;
  late FlutterSecureStorage storage;

  setUp(() {
    storageData = {};
    FlutterSecureStoragePlatform.instance = TestFlutterSecureStoragePlatform(
      storageData,
    );
    storage = const FlutterSecureStorage();
  });

  group('AuthState.login', () {
    test('stores the session and persists the refresh token', () async {
      final fakeService = FakeAuthService();
      final state = AuthState(fakeService, storage: storage);

      await state.login();

      expect(fakeService.loginCalls, 1);
      expect(state.accessToken, 'access-1');
      expect(storageData['zitadel_refresh_token'], 'refresh-1');
    });

    test('isLoggedIn is false until a user profile is also set', () async {
      final state = AuthState(FakeAuthService(), storage: storage);
      await state.login();
      expect(state.isLoggedIn, isFalse);

      state.setCurrentUser(
        AppUser(
          id: 'u1',
          tenantId: 't1',
          zitadelSubjectId: 'sub-1',
          displayName: 'Jane',
          email: 'jane@example.com',
          role: 'vendor',
          createdAt: DateTime.now(),
        ),
      );
      expect(state.isLoggedIn, isTrue);
    });
  });

  group('AuthState.tryRestoreSession', () {
    test('returns false when no refresh token is stored', () async {
      final state = AuthState(FakeAuthService(), storage: storage);
      expect(await state.tryRestoreSession(), isFalse);
    });

    test('restores a session using the stored refresh token', () async {
      storageData['zitadel_refresh_token'] = 'stored-refresh';
      final fakeService = FakeAuthService();
      final state = AuthState(fakeService, storage: storage);

      final restored = await state.tryRestoreSession();

      expect(restored, isTrue);
      expect(fakeService.lastRefreshTokenUsed, 'stored-refresh');
      expect(state.accessToken, 'access-refreshed');
    });

    test('clears the stored token and returns false if refresh fails', () async {
      storageData['zitadel_refresh_token'] = 'stale-refresh';
      final fakeService = FakeAuthService()..refreshError = Exception('invalid_grant');
      final state = AuthState(fakeService, storage: storage);

      final restored = await state.tryRestoreSession();

      expect(restored, isFalse);
      expect(storageData.containsKey('zitadel_refresh_token'), isFalse);
    });
  });

  group('AuthState.ensureFreshAccessToken', () {
    test('returns the current token without refreshing if not expired', () async {
      final fakeService = FakeAuthService()
        ..loginResult = AuthSession(
          accessToken: 'access-1',
          refreshToken: 'refresh-1',
          accessTokenExpiration: DateTime.now().add(const Duration(hours: 1)),
        );
      final state = AuthState(fakeService, storage: storage);
      await state.login();

      final token = await state.ensureFreshAccessToken();

      expect(token, 'access-1');
      expect(fakeService.refreshCalls, 0);
    });

    test('refreshes when the token is expired', () async {
      final fakeService = FakeAuthService()
        ..loginResult = AuthSession(
          accessToken: 'access-1',
          refreshToken: 'refresh-1',
          accessTokenExpiration: DateTime.now().subtract(const Duration(minutes: 1)),
        );
      final state = AuthState(fakeService, storage: storage);
      await state.login();

      final token = await state.ensureFreshAccessToken();

      expect(token, 'access-refreshed');
      expect(fakeService.refreshCalls, 1);
    });

    test('throws if not logged in', () async {
      final state = AuthState(FakeAuthService(), storage: storage);
      expect(() => state.ensureFreshAccessToken(), throwsStateError);
    });
  });

  group('AuthState.logout', () {
    test('clears the session, user, and stored refresh token', () async {
      final state = AuthState(FakeAuthService(), storage: storage);
      await state.login();
      state.setCurrentUser(
        AppUser(
          id: 'u1',
          tenantId: 't1',
          zitadelSubjectId: 'sub-1',
          displayName: 'Jane',
          email: 'jane@example.com',
          role: 'vendor',
          createdAt: DateTime.now(),
        ),
      );

      await state.logout();

      expect(state.isLoggedIn, isFalse);
      expect(state.accessToken, isNull);
      expect(state.currentUser, isNull);
      expect(storageData.containsKey('zitadel_refresh_token'), isFalse);
    });
  });
}
