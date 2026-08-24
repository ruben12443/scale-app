import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:scale_app/api/api_exception.dart';
import 'package:scale_app/api/core_api_client.dart';

void main() {
  group('CoreApiClient', () {
    test('listUsers sends a bearer token and parses the response', () async {
      Uri? gotUri;
      String? gotAuth;
      final client = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'the-token',
        httpClient: MockClient((req) async {
          gotUri = req.url;
          gotAuth = req.headers['Authorization'];
          return http.Response(
            jsonEncode([
              {
                'id': 'u1',
                'tenant_id': 't1',
                'zitadel_subject_id': 'sub-1',
                'display_name': 'Jane',
                'email': 'jane@example.com',
                'role': 'vendor',
                'created_at': '2026-08-23T14:30:00Z',
              },
            ]),
            200,
          );
        }),
      );

      final users = await client.listUsers();

      expect(gotUri.toString(), 'https://api.test/users');
      expect(gotAuth, 'Bearer the-token');
      expect(users, hasLength(1));
      expect(users.first.displayName, 'Jane');
    });

    test('createUser posts the expected JSON body', () async {
      Map<String, dynamic>? gotBody;
      final client = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'the-token',
        httpClient: MockClient((req) async {
          gotBody = jsonDecode(req.body) as Map<String, dynamic>;
          return http.Response(
            jsonEncode({
              'id': 'u1',
              'tenant_id': 't1',
              'zitadel_subject_id': 'sub-1',
              'display_name': 'Jane',
              'email': 'jane@example.com',
              'role': 'vendor',
              'created_at': '2026-08-23T14:30:00Z',
            }),
            201,
          );
        }),
      );

      await client.createUser(email: 'jane@example.com', displayName: 'Jane');

      expect(gotBody, {'email': 'jane@example.com', 'display_name': 'Jane'});
    });

    test(
      'throws ApiException with the server\'s error message on non-2xx',
      () async {
        final client = CoreApiClient(
          baseUrl: 'https://api.test',
          authToken: () async => null,
          httpClient: MockClient((req) async {
            return http.Response(jsonEncode({'error': 'not found'}), 404);
          }),
        );

        expect(
          () => client.listUsers(),
          throwsA(
            isA<ApiException>()
                .having((e) => e.statusCode, 'statusCode', 404)
                .having((e) => e.message, 'message', 'not found'),
          ),
        );
      },
    );

    test('omits Authorization header when no token is available', () async {
      String? gotAuth;
      var sawHeader = true;
      final client = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => null,
        httpClient: MockClient((req) async {
          sawHeader = req.headers.containsKey('Authorization');
          gotAuth = req.headers['Authorization'];
          return http.Response(jsonEncode([]), 200);
        }),
      );

      await client.listUsers();

      expect(sawHeader, isFalse);
      expect(gotAuth, isNull);
    });

    test(
      'createWeightTransaction parses the nested transaction + receipt_id',
      () async {
        final client = CoreApiClient(
          baseUrl: 'https://api.test',
          authToken: () async => 'tok',
          httpClient: MockClient((req) async {
            expect(req.url.path, '/transactions');
            return http.Response(
              jsonEncode({
                'transaction': {
                  'id': 'tx1',
                  'tenant_id': 't1',
                  'user_id': 'u1',
                  'product_id': 'p1',
                  'product_name': 'Tomatoes',
                  'pricing_type': 'per_kg',
                  'scale_id': 'scale-1',
                  'weight_grams': 1250,
                  'quantity': 0,
                  'unit_price_cents': 499,
                  'total_price_cents': 624,
                  'scale_status_code': '1',
                  'created_at': '2026-08-23T14:30:00Z',
                },
                'receipt_id': 'r1',
              }),
              201,
            );
          }),
        );

        final result = await client.createWeightTransaction(
          productId: 'p1',
          scaleId: 'scale-1',
          weightGrams: 1250,
          unitPriceCents: 499,
          totalPriceCents: 624,
          scaleStatusCode: '1',
        );

        expect(result.receiptId, 'r1');
        expect(result.transaction.productName, 'Tomatoes');
      },
    );

    test(
      'createPieceTransaction posts quantity instead of weight fields',
      () async {
        Map<String, dynamic>? gotBody;
        final client = CoreApiClient(
          baseUrl: 'https://api.test',
          authToken: () async => 'tok',
          httpClient: MockClient((req) async {
            gotBody = jsonDecode(req.body) as Map<String, dynamic>;
            return http.Response(
              jsonEncode({
                'transaction': {
                  'id': 'tx1',
                  'tenant_id': 't1',
                  'user_id': 'u1',
                  'product_id': 'p1',
                  'product_name': 'Eggs (dozen)',
                  'pricing_type': 'per_piece',
                  'scale_id': '',
                  'weight_grams': 0,
                  'quantity': 3,
                  'unit_price_cents': 550,
                  'total_price_cents': 1650,
                  'scale_status_code': '',
                  'created_at': '2026-08-23T14:30:00Z',
                },
                'receipt_id': 'r1',
              }),
              201,
            );
          }),
        );

        final result = await client.createPieceTransaction(
          productId: 'p1',
          quantity: 3,
          unitPriceCents: 550,
          totalPriceCents: 1650,
        );

        expect(gotBody, {
          'product_id': 'p1',
          'quantity': 3,
          'unit_price_cents': 550,
          'total_price_cents': 1650,
        });
        expect(result.transaction.quantity, 3);
        expect(result.transaction.isPerPiece, isTrue);
      },
    );

    test('reopenReceipt posts to /receipts/{id}/reopen', () async {
      Uri? gotUri;
      final client = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient((req) async {
          gotUri = req.url;
          return http.Response(
            jsonEncode({
              'id': 'r1',
              'tenant_id': 't1',
              'user_id': 'u1',
              'status': 'draft',
              'transaction_ids': [],
              'created_at': '2026-08-23T14:30:00Z',
              'finalized_at': null,
              'lines': [],
            }),
            200,
          );
        }),
      );

      final receipt = await client.reopenReceipt('r1');

      expect(gotUri.toString(), 'https://api.test/receipts/r1/reopen');
      expect(receipt.isDraft, isTrue);
    });

    test('deleteUser handles an empty 204 response body', () async {
      var calls = 0;
      final client = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient((req) async {
          calls++;
          return http.Response('', 204);
        }),
      );

      await client.deleteUser('u1');
      expect(calls, 1);
    });

    test('getMe fetches GET /me and parses the caller\'s own record', () async {
      String? gotPath;
      final client = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient((req) async {
          gotPath = req.url.path;
          return http.Response(
            jsonEncode({
              'id': 'u1',
              'tenant_id': 't1',
              'zitadel_subject_id': 'sub-1',
              'display_name': 'Jane',
              'email': 'jane@example.com',
              'role': 'admin',
              'created_at': '2026-08-23T14:30:00Z',
            }),
            200,
          );
        }),
      );

      final me = await client.getMe();

      expect(gotPath, '/me');
      expect(me.isAdmin, isTrue);
    });
  });
}
