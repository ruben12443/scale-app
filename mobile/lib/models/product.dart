/// Mirrors core-api's domain.Product JSON shape (see
/// backend/services/core-api/internal/domain/product.go).
class Product {
  final String id;
  final String tenantId;
  final String name;

  /// "per_kg" (weighed on a scale) or "per_piece" (counted).
  final String pricingType;

  /// Price per kg if [pricingType] is "per_kg", price per piece if
  /// "per_piece".
  final int unitPriceCents;
  final DateTime createdAt;

  const Product({
    required this.id,
    required this.tenantId,
    required this.name,
    required this.pricingType,
    required this.unitPriceCents,
    required this.createdAt,
  });

  bool get isPerPiece => pricingType == 'per_piece';

  factory Product.fromJson(Map<String, dynamic> json) {
    return Product(
      id: json['id'] as String,
      tenantId: json['tenant_id'] as String,
      name: json['name'] as String,
      pricingType: json['pricing_type'] as String,
      unitPriceCents: json['unit_price_cents'] as int,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
