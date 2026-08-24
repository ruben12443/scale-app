import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:scale_app/api/api_exception.dart';
import 'package:scale_app/api/scale_gateway_client.dart';

void main() {
  group('ScaleGatewayClient', () {
    test('listScales parses the status list', () async {
      final client = ScaleGatewayClient(
        baseUrl: 'http://192.168.1.50:8080',
        httpClient: MockClient((req) async {
          expect(req.url.path, '/scales');
          return http.Response(
            jsonEncode([
              {'id': 'scale-1', 'kind': 'dialog_raw_tcp', 'connected': true},
            ]),
            200,
          );
        }),
      );

      final scales = await client.listScales();
      expect(scales, hasLength(1));
      expect(scales.first.connected, isTrue);
    });

    test(
      'sendPrice posts price_per_kg_cents and parses the weigh result',
      () async {
        Map<String, dynamic>? gotBody;
        String? gotPath;
        final client = ScaleGatewayClient(
          baseUrl: 'http://192.168.1.50:8080',
          httpClient: MockClient((req) async {
            gotPath = req.url.path;
            gotBody = jsonDecode(req.body) as Map<String, dynamic>;
            return http.Response(
              jsonEncode({
                'scale_id': 'scale-1',
                'status_code': '1',
                'weight_grams': 1250,
                'price_cents': 624,
              }),
              200,
            );
          }),
        );

        final result = await client.sendPrice('scale-1', 499);

        expect(gotPath, '/scales/scale-1/transactions');
        expect(gotBody, {'price_per_kg_cents': 499});
        expect(result.weightGrams, 1250);
        expect(result.priceCents, 624);
      },
    );

    test('throws ApiException on a non-2xx response', () async {
      final client = ScaleGatewayClient(
        baseUrl: 'http://192.168.1.50:8080',
        httpClient: MockClient((req) async {
          return http.Response(jsonEncode({'error': 'scale unreachable'}), 502);
        }),
      );

      expect(
        () => client.sendPrice('scale-1', 499),
        throwsA(
          isA<ApiException>()
              .having((e) => e.statusCode, 'statusCode', 502)
              .having((e) => e.message, 'message', 'scale unreachable'),
        ),
      );
    });
  });
}
