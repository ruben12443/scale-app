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

  Future<ScaleWeighResult> sendPrice(String scaleId, int pricePerKgCents) async {
    final resp = await _http.post(
      _uri('/scales/$scaleId/transactions'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'price_per_kg_cents': pricePerKgCents}),
    );
    _checkStatus(resp);
    return ScaleWeighResult.fromJson(
      jsonDecode(resp.body) as Map<String, dynamic>,
    );
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
