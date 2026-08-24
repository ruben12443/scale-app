import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:scale_app/api/core_api_client.dart';
import 'package:scale_app/state/receipt_state.dart';

Map<String, dynamic> _draftReceiptJson({List<Map<String, dynamic>> lines = const []}) {
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
  'scale_id': 'scale-1',
  'weight_grams': 1250,
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

  test('addLine posts a transaction then refreshes the receipt', () async {
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

    await state.addLine(
      productId: 'p1',
      scaleId: 'scale-1',
      weightGrams: 1250,
      unitPriceCents: 499,
      totalPriceCents: 624,
      scaleStatusCode: '1',
    );

    expect(callCount, 2);
    expect(state.current?.lines, hasLength(1));
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

  test('finalizeReceipt updates current and returns the finalized receipt', () async {
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
  });

  test('emailReceipt throws if there is no current receipt', () async {
    final api = CoreApiClient(
      baseUrl: 'https://api.test',
      authToken: () async => 'tok',
      httpClient: MockClient((req) async => http.Response('', 200)),
    );
    final state = ReceiptState(api);

    expect(() => state.emailReceipt('a@b.com'), throwsStateError);
  });
}
