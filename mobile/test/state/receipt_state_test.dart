import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:scale_app/api/core_api_client.dart';
import 'package:scale_app/models/receipt.dart';
import 'package:scale_app/state/receipt_state.dart';

Map<String, dynamic> _draftReceiptJson({
  List<Map<String, dynamic>> lines = const [],
}) {
  return {
    'id': 'r1',
    'tenant_id': 't1',
    'user_id': 'u1',
    'status': 'draft',
    'transaction_ids': lines.map((l) => l['id']).toList(),
    'created_at': '2026-08-23T14:30:00Z',
    'finalized_at': null,
    'lines': lines,
  };
}

Map<String, dynamic> _txJson(String id) => {
  'id': id,
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
};

void main() {
  group('ReceiptState.refresh', () {
    test('populates current and clears loading/error on success', () async {
      final api = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient(
          (req) async => http.Response(jsonEncode(_draftReceiptJson()), 200),
        ),
      );
      final state = ReceiptState(api);

      await state.refresh();

      expect(state.loading, isFalse);
      expect(state.error, isNull);
      expect(state.current?.id, 'r1');
    });

    test('sets error and clears loading on failure', () async {
      final api = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient(
          (req) async => http.Response(jsonEncode({'error': 'boom'}), 500),
        ),
      );
      final state = ReceiptState(api);

      await state.refresh();

      expect(state.loading, isFalse);
      expect(state.error, contains('boom'));
      expect(state.current, isNull);
    });
  });

  test(
    'addWeightLine posts a transaction then refreshes the receipt',
    () async {
      var callCount = 0;
      final api = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient((req) async {
          callCount++;
          if (req.method == 'POST' && req.url.path == '/transactions') {
            return http.Response(
              jsonEncode({'transaction': _txJson('tx1'), 'receipt_id': 'r1'}),
              201,
            );
          }
          if (req.method == 'GET' && req.url.path == '/receipts/current') {
            return http.Response(
              jsonEncode(_draftReceiptJson(lines: [_txJson('tx1')])),
              200,
            );
          }
          throw StateError('unexpected request: ${req.method} ${req.url.path}');
        }),
      );
      final state = ReceiptState(api);

      await state.addWeightLine(
        productId: 'p1',
        scaleId: 'scale-1',
        weightGrams: 1250,
        unitPriceCents: 499,
        totalPriceCents: 624,
        scaleStatusCode: '1',
      );

      expect(callCount, 2);
      expect(state.current?.lines, hasLength(1));
    },
  );

  test('addPieceLine posts a piece transaction then refreshes', () async {
    var callCount = 0;
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async {
        callCount++;
        if (req.method == 'POST' && req.url.path == '/transactions') {
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['quantity'], 3);
          expect(body.containsKey('weight_grams'), isFalse);
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
        }
        if (req.method == 'GET' && req.url.path == '/receipts/current') {
          return http.Response(jsonEncode(_draftReceiptJson()), 200);
        }
        throw StateError('unexpected request: ${req.method} ${req.url.path}');
      }),
    );
    final state = ReceiptState(api);

    await state.addPieceLine(
      productId: 'p1',
      quantity: 3,
      unitPriceCents: 550,
      totalPriceCents: 1650,
    );

    expect(callCount, 2);
  });

  test('removeLine updates current from the response', () async {
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async {
        expect(req.method, 'DELETE');
        expect(req.url.path, '/receipts/current/lines/tx1');
        return http.Response(jsonEncode(_draftReceiptJson()), 200);
      }),
    );
    final state = ReceiptState(api);

    await state.removeLine('tx1');

    expect(state.current?.id, 'r1');
  });

  test(
    'finalizeReceipt updates current and returns the finalized receipt',
    () async {
      final api = CoreApiClient(
        baseUrl: 'https://api.test',
        authToken: () async => 'tok',
        httpClient: MockClient((req) async {
          return http.Response(
            jsonEncode({
              ..._draftReceiptJson(lines: [_txJson('tx1')]),
              'status': 'finalized',
              'number': 1,
              'rendered_text': 'text',
              'rendered_html': '<html></html>',
            }),
            200,
          );
        }),
      );
      final state = ReceiptState(api);

      final finalized = await state.finalizeReceipt();

      expect(finalized.isDraft, isFalse);
      expect(state.current?.isDraft, isFalse);
      expect(state.current?.number, 1);
    },
  );

  test('emailReceipt throws if there is no current receipt', () async {
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async => http.Response('', 200)),
    );
    final state = ReceiptState(api);

    expect(() => state.emailReceipt('a@b.com'), throwsStateError);
  });

  test('emailReceipt refreshes so the receipt shows as sent', () async {
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async {
        if (req.method == 'POST' && req.url.path == '/receipts/r1/email') {
          return http.Response('', 204);
        }
        if (req.method == 'GET' && req.url.path == '/receipts/current') {
          return http.Response(
            jsonEncode({
              ..._draftReceiptJson(lines: [_txJson('tx1')]),
              'status': 'sent',
              'number': 1,
              'sent_to': 'a@b.com',
            }),
            200,
          );
        }
        throw StateError('unexpected request: ${req.method} ${req.url.path}');
      }),
    );
    final state = ReceiptState(api);
    state.current = Receipt.fromJson({
      ..._draftReceiptJson(lines: [_txJson('tx1')]),
      'status': 'finalized',
      'number': 1,
    });

    await state.emailReceipt('a@b.com');

    expect(state.current?.isSent, isTrue);
    expect(state.current?.sentTo, 'a@b.com');
  });

  test('reopenReceipt throws if there is no current receipt', () async {
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async => http.Response('', 200)),
    );
    final state = ReceiptState(api);

    expect(() => state.reopenReceipt(), throwsStateError);
  });

  test('reopenReceipt updates current from the response', () async {
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/receipts/r1/reopen');
        return http.Response(jsonEncode(_draftReceiptJson()), 200);
      }),
    );
    final state = ReceiptState(api);
    state.current = Receipt.fromJson({
      ..._draftReceiptJson(),
      'status': 'finalized',
      'number': 1,
    });

    await state.reopenReceipt();

    expect(state.current?.isDraft, isTrue);
  });
}
