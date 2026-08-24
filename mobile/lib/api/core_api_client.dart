import 'dart:convert';

import 'package:http/http.dart' as http;

import '../models/product.dart';
import '../models/receipt.dart';
import '../models/transaction.dart';
import '../models/user.dart';
import 'api_exception.dart';

/// Result of POST /transactions: the recorded transaction, and which draft
/// receipt it was appended to.
typedef CreateTransactionResult = ({
  ScaleTransaction transaction,
  String receiptId,
});

/// Result of POST /receipts/{id}/payment.
typedef CreatePaymentResult = ({
  String paymentId,
  String paymentIntentId,
  String clientSecret,
  int amountCents,
});

/// Client for core-api's HTTP surface (see
/// backend/services/core-api/README.md for the full endpoint list).
///
/// [authToken] is called fresh before every request and may be async, so a
/// caller can refresh an about-to-expire token (see
/// AuthState.ensureFreshAccessToken) as part of producing it, rather than
/// requests racing an expiry check against a plain in-memory value.
class CoreApiClient {
  final String baseUrl;
  final http.Client _http;
  final Future<String?> Function() authToken;

  CoreApiClient({
    required this.baseUrl,
    required this.authToken,
    http.Client? httpClient,
  }) : _http = httpClient ?? http.Client();

  Uri _uri(String path) => Uri.parse('$baseUrl$path');

  Future<Map<String, String>> _headers() async {
    final token = await authToken();
    return {
      'Content-Type': 'application/json',
      if (token != null) 'Authorization': 'Bearer $token',
    };
  }

  Future<dynamic> _send(String method, String path, {Object? body}) async {
    final http.Response resp;
    final uri = _uri(path);
    final encodedBody = body == null ? null : jsonEncode(body);
    final headers = await _headers();
    switch (method) {
      case 'GET':
        resp = await _http.get(uri, headers: headers);
      case 'POST':
        resp = await _http.post(uri, headers: headers, body: encodedBody);
      case 'DELETE':
        resp = await _http.delete(uri, headers: headers);
      default:
        throw ArgumentError('unsupported method $method');
    }

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

    if (resp.body.isEmpty) return null;
    return jsonDecode(resp.body);
  }

  // --- Users ---

  /// GET /me — the caller's own user record (tenant, role, display name).
  /// The only way a client learns this after logging in via Zitadel.
  Future<AppUser> getMe() async {
    final data = await _send('GET', '/me');
    return AppUser.fromJson(data as Map<String, dynamic>);
  }

  // --- Users (admin only) ---

  Future<List<AppUser>> listUsers() async {
    final data = await _send('GET', '/users') as List<dynamic>;
    return data
        .map((u) => AppUser.fromJson(u as Map<String, dynamic>))
        .toList();
  }

  Future<AppUser> createUser({
    required String email,
    required String displayName,
  }) async {
    final data = await _send(
      'POST',
      '/users',
      body: {'email': email, 'display_name': displayName},
    );
    return AppUser.fromJson(data as Map<String, dynamic>);
  }

  Future<void> deleteUser(String id) => _send('DELETE', '/users/$id');

  // --- Products ---

  Future<List<Product>> listProducts() async {
    final data = await _send('GET', '/products') as List<dynamic>;
    return data
        .map((p) => Product.fromJson(p as Map<String, dynamic>))
        .toList();
  }

  // --- Transactions / draft receipt ---

  Future<CreateTransactionResult> createTransaction({
    required String productId,
    required String scaleId,
    required int weightGrams,
    required int unitPriceCents,
    required int totalPriceCents,
    required String scaleStatusCode,
  }) async {
    final data = await _send(
      'POST',
      '/transactions',
      body: {
        'product_id': productId,
        'scale_id': scaleId,
        'weight_grams': weightGrams,
        'unit_price_cents': unitPriceCents,
        'total_price_cents': totalPriceCents,
        'scale_status_code': scaleStatusCode,
      },
    ) as Map<String, dynamic>;
    return (
      transaction: ScaleTransaction.fromJson(
        data['transaction'] as Map<String, dynamic>,
      ),
      receiptId: data['receipt_id'] as String,
    );
  }

  Future<Receipt> getCurrentReceipt() async {
    final data = await _send('GET', '/receipts/current');
    return Receipt.fromJson(data as Map<String, dynamic>);
  }

  Future<Receipt> removeReceiptLine(String transactionId) async {
    final data = await _send(
      'DELETE',
      '/receipts/current/lines/$transactionId',
    );
    return Receipt.fromJson(data as Map<String, dynamic>);
  }

  Future<Receipt> finalizeReceipt() async {
    final data = await _send('POST', '/receipts/current/finalize');
    return Receipt.fromJson(data as Map<String, dynamic>);
  }

  Future<void> emailReceipt(String receiptId, String to) =>
      _send('POST', '/receipts/$receiptId/email', body: {'to': to});

  // --- Payments ---

  Future<String> getPaymentConnectionToken() async {
    final data = await _send(
      'POST',
      '/payments/connection-token',
    ) as Map<String, dynamic>;
    return data['secret'] as String;
  }

  Future<CreatePaymentResult> createPayment(String receiptId) async {
    final data = await _send(
      'POST',
      '/receipts/$receiptId/payment',
    ) as Map<String, dynamic>;
    return (
      paymentId: data['payment_id'] as String,
      paymentIntentId: data['payment_intent_id'] as String,
      clientSecret: data['client_secret'] as String,
      amountCents: data['amount_cents'] as int,
    );
  }
}
