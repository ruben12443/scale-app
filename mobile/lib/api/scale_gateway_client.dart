import 'dart:convert';

import 'package:http/http.dart' as http;

import '../models/scale_status.dart';
import 'api_exception.dart';

/// Client for the local scale-gateway service running on the same network
/// as the scale(s) at this stall (see
/// backend/services/scale-gateway/README.md). No authentication: it's a
/// trusted local-network service, not exposed to the internet.
class ScaleGatewayClient {
  final String baseUrl;
  final http.Client _http;

  ScaleGatewayClient({required this.baseUrl, http.Client? httpClient})
    : _http = httpClient ?? http.Client();

  Uri _uri(String path) => Uri.parse('$baseUrl$path');

  Future<List<ScaleStatus>> listScales() async {
    final resp = await _http.get(_uri('/scales'));
    _checkStatus(resp);
    final data = jsonDecode(resp.body) as List<dynamic>;
    return data
        .map((s) => ScaleStatus.fromJson(s as Map<String, dynamic>))
        .toList();
  }

  Future<ScaleWeighResult> sendPrice(
    String scaleId,
    int pricePerKgCents, {
    required String holderId,
  }) async {
    final resp = await _http.post(
      _uri('/scales/$scaleId/transactions'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'price_per_kg_cents': pricePerKgCents,
        'holder_id': holderId,
      }),
    );
    _checkStatus(resp);
    return ScaleWeighResult.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
    );
  }

  /// Claims exclusive use of a scale for [holderId], so no other vendor can
  /// weigh on it at the same time. Re-claiming with the same [holderId] is a
  /// no-op renewal that extends the claim; throws [ApiException] with
  /// statusCode 409 if another vendor currently holds it.
  Future<void> claimScale(
    String scaleId, {
    required String holderId,
    String? holderName,
  }) async {
    final body = <String, String>{'holder_id': holderId};
    if (holderName != null) body['holder_name'] = holderName;
    final resp = await _http.post(
      _uri('/scales/$scaleId/claim'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode(body),
    );
    _checkStatus(resp);
  }

  /// Releases [holderId]'s claim on a scale, if it still holds one. Always
  /// succeeds — releasing is a best-effort cleanup action, not something
  /// callers need to retry or handle failure for.
  Future<void> releaseScale(String scaleId, {required String holderId}) async {
    final resp = await _http.post(
      _uri('/scales/$scaleId/release'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'holder_id': holderId}),
    );
    _checkStatus(resp);
  }

  void _checkStatus(http.Response resp) {
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      String message = resp.body;
      try {
        final decoded = jsonDecode(resp.body);
        if (decoded is Map && decoded['error'] is String) {
          message = decoded['error'] as String;
        }
      } catch (_) {
        // Body wasn't JSON; fall back to the raw text set above.
      }
      throw ApiException(message, statusCode: resp.statusCode);
    }
  }
}
