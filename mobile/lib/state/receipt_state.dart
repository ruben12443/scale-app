import 'package:flutter/foundation.dart';

import '../api/core_api_client.dart';
import '../models/receipt.dart';

/// Wraps the caller's current draft receipt against core-api, so any
/// screen can observe it (e.g. a running total badge) without re-fetching.
class ReceiptState extends ChangeNotifier {
  final CoreApiClient _api;

  Receipt? current;
  bool loading = false;
  String? error;

  ReceiptState(this._api);

  Future<void> refresh() async {
    loading = true;
    error = null;
    notifyListeners();
    try {
      current = await _api.getCurrentReceipt();
    } catch (e) {
      error = e.toString();
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  /// Records a scale-approved transaction and appends it to the draft
  /// receipt (mirrors POST /transactions's own behavior), then refreshes.
  Future<void> addLine({
    required String productId,
    required String scaleId,
    required int weightGrams,
    required int unitPriceCents,
    required int totalPriceCents,
    required String scaleStatusCode,
  }) async {
    await _api.createTransaction(
      productId: productId,
      scaleId: scaleId,
      weightGrams: weightGrams,
      unitPriceCents: unitPriceCents,
      totalPriceCents: totalPriceCents,
      scaleStatusCode: scaleStatusCode,
    );
    await refresh();
  }

  Future<void> removeLine(String transactionId) async {
    current = await _api.removeReceiptLine(transactionId);
    notifyListeners();
  }

  Future<Receipt> finalizeReceipt() async {
    final finalized = await _api.finalizeReceipt();
    current = finalized;
    notifyListeners();
    return finalized;
  }

  Future<void> emailReceipt(String to) async {
    final receipt = current;
    if (receipt == null) throw StateError('no current receipt');
    await _api.emailReceipt(receipt.id, to);
  }
}
