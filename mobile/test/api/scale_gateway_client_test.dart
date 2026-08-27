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

    test('sendPrice posts price_per_kg_cents and holder_id, and parses the weigh result', () async {
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

      final result = await client.sendPrice(
        'scale-1',
        499,
        holderId: 'vendor-1',
      );

      expect(gotPath, '/scales/scale-1/transactions');
      expect(gotBody, {'price_per_kg_cents': 499, 'holder_id': 'vendor-1'});
      expect(result.weightGrams, 1250);
      expect(result.priceCents, 624);
    });

    test('throws ApiException on a non-2xx response', () async {
      final client = ScaleGatewayClient(
        baseUrl: 'http://192.168.1.50:8080',
        httpClient: MockClient((req) async {
          return http.Response(jsonEncode({'error': 'scale unreachable'}), 502);
        }),
      );

      expect(
        () => client.sendPrice('scale-1', 499, holderId: 'vendor-1'),
        throwsA(
          isA<ApiException>()
              .having((e) => e.statusCode, 'statusCode', 502)
              .having((e) => e.message, 'message', 'scale unreachable'),
        ),
      );
    });

    test('claimScale posts holder_id and holder_name', () async {
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
              'holder_id': 'vendor-1',
              'holder_name': 'Alice',
              'expires_at': '2026-01-01T00:00:20Z',
            }),
            200,
          );
        }),
      );

      await client.claimScale(
        'scale-1',
        holderId: 'vendor-1',
        holderName: 'Alice',
      );

      expect(gotPath, '/scales/scale-1/claim');
      expect(gotBody, {'holder_id': 'vendor-1', 'holder_name': 'Alice'});
    });

    test('claimScale throws a 409 ApiException when already held', () async {
      final client = ScaleGatewayClient(
        baseUrl: 'http://192.168.1.50:8080',
        httpClient: MockClient((req) async {
          return http.Response(
            jsonEncode({
              'error': 'scale is in use by another vendor',
              'held_by_id': 'vendor-2',
              'held_by_name': 'Bob',
            }),
            409,
          );
        }),
      );

      expect(
        () => client.claimScale('scale-1', holderId: 'vendor-1'),
        throwsA(
          isA<ApiException>().having((e) => e.statusCode, 'statusCode', 409),
        ),
      );
    });

    test('releaseScale posts holder_id', () async {
      Map<String, dynamic>? gotBody;
      String? gotPath;
      final client = ScaleGatewayClient(
        baseUrl: 'http://192.168.1.50:8080',
        httpClient: MockClient((req) async {
          gotPath = req.url.path;
          gotBody = jsonDecode(req.body) as Map<String, dynamic>;
          return http.Response(jsonEncode({'released': true}), 200);
        }),
      );

      await client.releaseScale('scale-1', holderId: 'vendor-1');

      expect(gotPath, '/scales/scale-1/release');
      expect(gotBody, {'holder_id': 'vendor-1'});
    });
  });
}
