import 'package:flutter_test/flutter_test.dart';
import 'package:scale_app/models/product.dart';
import 'package:scale_app/models/receipt.dart';
import 'package:scale_app/models/scale_status.dart';
import 'package:scale_app/models/transaction.dart';
import 'package:scale_app/models/user.dart';

void main() {
  group('AppUser', () {
    test('fromJson parses core-api\'s field names', () {
      final user = AppUser.fromJson({
        'id': 'u1',
        'tenant_id': 't1',
        'zitadel_subject_id': 'sub-1',
        'display_name': 'Jane Vendor',
        'email': 'jane@example.com',
        'role': 'admin',
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(user.id, 'u1');
      expect(user.tenantId, 't1');
      expect(user.isAdmin, isTrue);
      expect(user.createdAt, DateTime.utc(2026, 8, 23, 14, 30));
    });

    test('isAdmin is false for a vendor role', () {
      final user = AppUser.fromJson({
        'id': 'u1',
        'tenant_id': 't1',
        'zitadel_subject_id': 'sub-1',
        'display_name': 'Jane Vendor',
        'email': 'jane@example.com',
        'role': 'vendor',
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(user.isAdmin, isFalse);
    });
  });

  group('Product', () {
    test('fromJson parses a per-kg product', () {
      final p = Product.fromJson({
        'id': 'p1',
        'tenant_id': 't1',
        'name': 'Tomatoes',
        'pricing_type': 'per_kg',
        'unit_price_cents': 499,
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(p.name, 'Tomatoes');
      expect(p.unitPriceCents, 499);
      expect(p.isPerPiece, isFalse);
    });

    test('fromJson parses a per-piece product', () {
      final p = Product.fromJson({
        'id': 'p2',
        'tenant_id': 't1',
        'name': 'Eggs (dozen)',
        'pricing_type': 'per_piece',
        'unit_price_cents': 550,
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(p.isPerPiece, isTrue);
      expect(p.unitPriceCents, 550);
    });
  });

  group('ScaleTransaction', () {
    test('fromJson parses a per-kg (weighed) line', () {
      final tx = ScaleTransaction.fromJson({
        'id': 'tx1',
        'tenant_id': 't1',
        'user_id': 'u1',
        'product_id': 'p1',
        'product_name': 'Tomatoes',
        'pricing_type': 'per_kg',
        'scale_id': 'scale-1',
        'weight_grams': 1250,
        'quantity': 0,
        'unit_price_cents': 499,
        'total_price_cents': 624,
        'scale_status_code': '1',
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(tx.weightGrams, 1250);
      expect(tx.totalPriceCents, 624);
      expect(tx.isPerPiece, isFalse);
    });

    test('fromJson parses a per-piece (counted) line', () {
      final tx = ScaleTransaction.fromJson({
        'id': 'tx1',
        'tenant_id': 't1',
        'user_id': 'u1',
        'product_id': 'p1',
        'product_name': 'Eggs (dozen)',
        'pricing_type': 'per_piece',
        'scale_id': '',
        'weight_grams': 0,
        'quantity': 3,
        'unit_price_cents': 550,
        'total_price_cents': 1650,
        'scale_status_code': '',
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(tx.quantity, 3);
      expect(tx.totalPriceCents, 1650);
      expect(tx.isPerPiece, isTrue);
    });

    test('formattedWeight matches the backend\'s rendering', () {
      final tx = ScaleTransaction.fromJson({
        'id': 'tx1',
        'tenant_id': 't1',
        'user_id': 'u1',
        'product_id': 'p1',
        'product_name': 'Tomatoes',
        'pricing_type': 'per_kg',
        'scale_id': 'scale-1',
        'weight_grams': 1250,
        'quantity': 0,
        'unit_price_cents': 499,
        'total_price_cents': 624,
        'scale_status_code': '1',
        'created_at': '2026-08-23T14:30:00Z',
      });
      expect(tx.formattedWeight, '1.250 kg');
    });

    test('formatCents matches the backend\'s rendering', () {
      expect(ScaleTransaction.formatCents(624), '6.24');
      expect(ScaleTransaction.formatCents(5), '0.05');
      expect(ScaleTransaction.formatCents(100), '1.00');
    });
  });

  group('Receipt', () {
    test('fromJson resolves lines and computes total', () {
      final receipt = Receipt.fromJson({
        'id': 'r1',
        'tenant_id': 't1',
        'user_id': 'u1',
        'status': 'draft',
        'transaction_ids': ['tx1', 'tx2'],
        'created_at': '2026-08-23T14:30:00Z',
        'finalized_at': null,
        'lines': [
          {
            'id': 'tx1',
            'tenant_id': 't1',
            'user_id': 'u1',
            'product_id': 'p1',
            'product_name': 'Tomatoes',
            'pricing_type': 'per_kg',
            'scale_id': 'scale-1',
            'weight_grams': 1250,
            'quantity': 0,
            'unit_price_cents': 499,
            'total_price_cents': 624,
            'scale_status_code': '1',
            'created_at': '2026-08-23T14:30:00Z',
          },
          {
            'id': 'tx2',
            'tenant_id': 't1',
            'user_id': 'u1',
            'product_id': 'p2',
            'product_name': 'Potatoes',
            'pricing_type': 'per_kg',
            'scale_id': 'scale-1',
            'weight_grams': 2000,
            'quantity': 0,
            'unit_price_cents': 199,
            'total_price_cents': 398,
            'scale_status_code': '1',
            'created_at': '2026-08-23T14:30:00Z',
          },
        ],
      });

      expect(receipt.isDraft, isTrue);
      expect(receipt.lines, hasLength(2));
      expect(receipt.totalCents, 1022);
      expect(receipt.finalizedAt, isNull);
      expect(receipt.number, 0);
    });

    test('fromJson parses a finalized receipt with rendered output', () {
      final receipt = Receipt.fromJson({
        'id': 'r1',
        'tenant_id': 't1',
        'user_id': 'u1',
        'status': 'finalized',
        'number': 42,
        'transaction_ids': [],
        'created_at': '2026-08-23T14:30:00Z',
        'finalized_at': '2026-08-23T14:35:00Z',
        'lines': [],
        'rendered_text': 'plain text receipt',
        'rendered_html': '<html></html>',
      });

      expect(receipt.isDraft, isFalse);
      expect(receipt.isFinalized, isTrue);
      expect(receipt.number, 42);
      expect(receipt.finalizedAt, DateTime.utc(2026, 8, 23, 14, 35));
      expect(receipt.renderedText, 'plain text receipt');
      expect(receipt.renderedHtml, '<html></html>');
    });

    test('fromJson parses a sent receipt as locked', () {
      final receipt = Receipt.fromJson({
        'id': 'r1',
        'tenant_id': 't1',
        'user_id': 'u1',
        'status': 'sent',
        'number': 42,
        'transaction_ids': [],
        'created_at': '2026-08-23T14:30:00Z',
        'finalized_at': '2026-08-23T14:35:00Z',
        'sent_at': '2026-08-23T14:36:00Z',
        'sent_to': 'customer@example.com',
        'lines': [],
      });

      expect(receipt.isSent, isTrue);
      expect(receipt.isFinalized, isFalse);
      expect(receipt.sentTo, 'customer@example.com');
      expect(receipt.sentAt, DateTime.utc(2026, 8, 23, 14, 36));
    });
  });

  group('ScaleStatus', () {
    test('fromJson parses connected scale', () {
      final s = ScaleStatus.fromJson({
        'id': 'scale-1',
        'kind': 'dialog_raw_tcp',
        'connected': true,
      });
      expect(s.connected, isTrue);
      expect(s.lastError, isNull);
    });

    test('fromJson parses a disconnected scale with an error', () {
      final s = ScaleStatus.fromJson({
        'id': 'scale-1',
        'kind': 'dialog_raw_tcp',
        'connected': false,
        'last_error': 'connection refused',
      });
      expect(s.connected, isFalse);
      expect(s.lastError, 'connection refused');
    });
  });

  group('ScaleWeighResult', () {
    test('fromJson parses scale-gateway\'s response shape', () {
      final r = ScaleWeighResult.fromJson({
        'scale_id': 'scale-1',
        'status_code': '1',
        'weight_grams': 1250,
        'price_cents': 624,
      });
      expect(r.weightGrams, 1250);
      expect(r.priceCents, 624);
    });
  });
}
