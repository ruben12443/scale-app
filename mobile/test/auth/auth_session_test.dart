import 'package:flutter_test/flutter_test.dart';
import 'package:scale_app/auth/auth_service.dart';

void main() {
  group('AuthSession.isExpired', () {
    test('is false with no expiration set', () {
      const session = AuthSession(accessToken: 'tok');
      expect(session.isExpired(), isFalse);
    });

    test('is false well before expiry', () {
      final session = AuthSession(
        accessToken: 'tok',
        accessTokenExpiration: DateTime.now().add(const Duration(minutes: 10)),
      );
      expect(session.isExpired(), isFalse);
    });

    test('is true once within the skew window of expiry', () {
      final session = AuthSession(
        accessToken: 'tok',
        accessTokenExpiration: DateTime.now().add(const Duration(seconds: 10)),
      );
      expect(session.isExpired(skew: const Duration(seconds: 30)), isTrue);
    });

    test('is true once past expiry', () {
      final session = AuthSession(
        accessToken: 'tok',
        accessTokenExpiration: DateTime.now().subtract(const Duration(minutes: 1)),
      );
      expect(session.isExpired(), isTrue);
    });
  });
}
