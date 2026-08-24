/// Mirrors scale-gateway's scaleStatus JSON shape (see
/// backend/services/scale-gateway/internal/gateway/server.go).
class ScaleStatus {
  final String id;
  final String kind;
  final bool connected;
  final String? lastError;

  const ScaleStatus({
    required this.id,
    required this.kind,
    required this.connected,
    this.lastError,
  });

  factory ScaleStatus.fromJson(Map<String, dynamic> json) {
    return ScaleStatus(
      id: json['id'] as String,
      kind: json['kind'] as String,
      connected: json['connected'] as bool,
      lastError: json['last_error'] as String?,
    );
  }
}

/// Mirrors scale-gateway's sendTransactionResponse JSON shape.
class ScaleWeighResult {
  final String scaleId;
  final String statusCode;
  final int weightGrams;
  final int priceCents;

  const ScaleWeighResult({
    required this.scaleId,
    required this.statusCode,
    required this.weightGrams,
    required this.priceCents,
  });

  factory ScaleWeighResult.fromJson(Map<String, dynamic> json) {
    return ScaleWeighResult(
      scaleId: json['scale_id'] as String,
      statusCode: json['status_code'] as String,
      weightGrams: json['weight_grams'] as int,
      priceCents: json['price_cents'] as int,
    );
  }
}
