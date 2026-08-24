import 'transaction.dart';

/// Mirrors core-api's api.receiptResponse shape: a domain.Receipt with its
/// transaction lines resolved (see
/// backend/services/core-api/internal/api/receipt_handlers.go).
class Receipt {
  final String id;
  final String tenantId;
  final String userId;
  final String status; // "draft" or "finalized"
  final int number;
  final List<String> transactionIds;
  final DateTime createdAt;
  final DateTime? finalizedAt;
  final List<ScaleTransaction> lines;

  /// Non-null only on the response from POST /receipts/current/finalize.
  final String? renderedText;
  final String? renderedHtml;

  const Receipt({
    required this.id,
    required this.tenantId,
    required this.userId,
    required this.status,
    required this.number,
    required this.transactionIds,
    required this.createdAt,
    required this.finalizedAt,
    required this.lines,
    this.renderedText,
    this.renderedHtml,
  });

  bool get isDraft => status == 'draft';

  int get totalCents => lines.fold(0, (sum, l) => sum + l.totalPriceCents);

  factory Receipt.fromJson(Map<String, dynamic> json) {
    return Receipt(
      id: json['id'] as String,
      tenantId: json['tenant_id'] as String,
      userId: json['user_id'] as String,
      status: json['status'] as String,
      number: json['number'] as int? ?? 0,
      transactionIds: (json['transaction_ids'] as List<dynamic>? ?? [])
          .cast<String>(),
      createdAt: DateTime.parse(json['created_at'] as String),
      finalizedAt: json['finalized_at'] == null
          ? null
          : DateTime.parse(json['finalized_at'] as String),
      lines: (json['lines'] as List<dynamic>? ?? [])
          .map((l) => ScaleTransaction.fromJson(l as Map<String, dynamic>))
          .toList(),
      renderedText: json['rendered_text'] as String?,
      renderedHtml: json['rendered_html'] as String?,
    );
  }
}
