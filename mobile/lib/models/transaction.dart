/// Mirrors core-api's domain.Transaction JSON shape (see
/// backend/services/core-api/internal/domain/transaction.go). This is the
/// immutable record of one scale-approved weigh event.
class ScaleTransaction {
  final String id;
  final String tenantId;
  final String userId;
  final String productId;
  final String productName;
  final String scaleId;
  final int weightGrams;
  final int unitPriceCents;
  final int totalPriceCents;
  final String scaleStatusCode;
  final DateTime createdAt;

  const ScaleTransaction({
    required this.id,
    required this.tenantId,
    required this.userId,
    required this.productId,
    required this.productName,
    required this.scaleId,
    required this.weightGrams,
    required this.unitPriceCents,
    required this.totalPriceCents,
    required this.scaleStatusCode,
    required this.createdAt,
  });

  factory ScaleTransaction.fromJson(Map<String, dynamic> json) {
    return ScaleTransaction(
      id: json['id'] as String,
      tenantId: json['tenant_id'] as String,
      userId: json['user_id'] as String,
      productId: json['product_id'] as String,
      productName: json['product_name'] as String,
      scaleId: json['scale_id'] as String,
      weightGrams: json['weight_grams'] as int,
      unitPriceCents: json['unit_price_cents'] as int,
      totalPriceCents: json['total_price_cents'] as int,
      scaleStatusCode: json['scale_status_code'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  /// e.g. "1.250 kg", matching the backend's receipt rendering.
  String get formattedWeight {
    final kg = weightGrams ~/ 1000;
    final g = (weightGrams % 1000).toString().padLeft(3, '0');
    return '$kg.$g kg';
  }

  /// e.g. "4.99", matching the backend's receipt rendering.
  static String formatCents(int cents) {
    final whole = cents ~/ 100;
    final fraction = (cents % 100).toString().padLeft(2, '0');
    return '$whole.$fraction';
  }
}
