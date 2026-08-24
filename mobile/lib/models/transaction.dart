/// Mirrors core-api's domain.Transaction JSON shape (see
/// backend/services/core-api/internal/domain/transaction.go). This is the
/// immutable record of one sale line: either a scale-approved weigh event
/// ("per_kg", [weightGrams] set) or a counted line ("per_piece", [quantity]
/// set).
class ScaleTransaction {
  final String id;
  final String tenantId;
  final String userId;
  final String productId;
  final String productName;
  final String pricingType;
  final String scaleId;
  final int weightGrams;
  final int quantity;
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
    required this.pricingType,
    required this.scaleId,
    required this.weightGrams,
    required this.quantity,
    required this.unitPriceCents,
    required this.totalPriceCents,
    required this.scaleStatusCode,
    required this.createdAt,
  });

  bool get isPerPiece => pricingType == 'per_piece';

  factory ScaleTransaction.fromJson(Map<String, dynamic> json) {
    return ScaleTransaction(
      id: json['id'] as String,
      tenantId: json['tenant_id'] as String,
      userId: json['user_id'] as String,
      productId: json['product_id'] as String,
      productName: json['product_name'] as String,
      pricingType: json['pricing_type'] as String,
      scaleId: json['scale_id'] as String? ?? '',
      weightGrams: json['weight_grams'] as int? ?? 0,
      quantity: json['quantity'] as int? ?? 0,
      unitPriceCents: json['unit_price_cents'] as int,
      totalPriceCents: json['total_price_cents'] as int,
      scaleStatusCode: json['scale_status_code'] as String? ?? '',
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
