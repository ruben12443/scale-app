/// Mirrors core-api's domain.Product JSON shape (see
/// backend/services/core-api/internal/domain/product.go).
class Product {
  final String id;
  final String tenantId;
  final String name;
  final int pricePerKgCents;
  final DateTime createdAt;

  const Product({
    required this.id,
    required this.tenantId,
    required this.name,
    required this.pricePerKgCents,
    required this.createdAt,
  });

  factory Product.fromJson(Map<String, dynamic> json) {
    return Product(
      id: json['id'] as String,
      tenantId: json['tenant_id'] as String,
      name: json['name'] as String,
      pricePerKgCents: json['price_per_kg_cents'] as int,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
