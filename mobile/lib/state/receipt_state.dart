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

  /// Records a scale-approved weigh event and appends it to the draft
  /// receipt (mirrors POST /transactions's own behavior), then refreshes.
  Future<void> addWeightLine({
    required String productId,
    required String scaleId,
    required int weightGrams,
    required int unitPriceCents,
    required int totalPriceCents,
    required String scaleStatusCode,
  }) async {
    await _api.createWeightTransaction(
      productId: productId,
      scaleId: scaleId,
      weightGrams: weightGrams,
      unitPriceCents: unitPriceCents,
      totalPriceCents: totalPriceCents,
      scaleStatusCode: scaleStatusCode,
    );
    await refresh();
  }

  /// Records a counted (per-piece) line and appends it to the draft
  /// receipt, then refreshes. Never touches a scale.
  Future<void> addPieceLine({
    required String productId,
    required int quantity,
    required int unitPriceCents,
    required int totalPriceCents,
  }) async {
    await _api.createPieceTransaction(
      productId: productId,
      quantity: quantity,
      unitPriceCents: unitPriceCents,
      totalPriceCents: totalPriceCents,
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
    await refresh();
  }

  /// Puts a finalized (not yet sent) receipt back into draft.
  Future<void> reopenReceipt() async {
    final receipt = current;
    if (receipt == null) throw StateError('no current receipt');
    current = await _api.reopenReceipt(receipt.id);
    notifyListeners();
  }
}
