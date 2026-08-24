import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api/core_api_client.dart';
import '../api/scale_gateway_client.dart';
import '../models/product.dart';
import '../models/scale_status.dart';
import '../models/transaction.dart';
import '../state/receipt_state.dart';

/// The core sell flow for one scale. A per-kg product sends its price to
/// the scale, which weighs and computes the total on its own certified
/// display, verified here before locking it in. A per-piece product skips
/// the scale entirely: pick a quantity, and the app computes the ordinary
/// quantity x price total itself (no legal-metrology concern, since
/// nothing is physically measured).
class SellScreen extends StatefulWidget {
  final ScaleStatus scale;
  final ScaleGatewayClient gatewayClient;
  final CoreApiClient coreApiClient;

  const SellScreen({
    super.key,
    required this.scale,
    required this.gatewayClient,
    required this.coreApiClient,
  });

  @override
  State<SellScreen> createState() => _SellScreenState();
}

class _SellScreenState extends State<SellScreen> {
  late Future<List<Product>> _productsFuture;
  final _searchController = TextEditingController();
  String _query = '';

  ScaleWeighResult? _pendingResult;
  Product? _pendingProduct;
  Product? _pendingPieceProduct;
  int _pieceQuantity = 1;

  bool _weighing = false;
  bool _locking = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _productsFuture = widget.coreApiClient.listProducts();
    _searchController.addListener(() {
      setState(() => _query = _searchController.text);
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _pickProduct(Product product) async {
    if (product.isPerPiece) {
      setState(() {
        _pendingPieceProduct = product;
        _pieceQuantity = 1;
        _error = null;
      });
      return;
    }

    setState(() {
      _weighing = true;
      _error = null;
    });
    try {
      final result = await widget.gatewayClient.sendPrice(
        widget.scale.id,
        product.unitPriceCents,
      );
      setState(() {
        _pendingProduct = product;
        _pendingResult = result;
      });
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _weighing = false);
    }
  }

  Future<void> _confirmAndLockIn() async {
    final product = _pendingProduct;
    final result = _pendingResult;
    if (product == null || result == null) return;

    setState(() {
      _locking = true;
      _error = null;
    });
    try {
      await context.read<ReceiptState>().addWeightLine(
        productId: product.id,
        scaleId: widget.scale.id,
        weightGrams: result.weightGrams,
        unitPriceCents: product.unitPriceCents,
        totalPriceCents: result.priceCents,
        scaleStatusCode: result.statusCode,
      );
      if (mounted) {
        setState(() {
          _pendingProduct = null;
          _pendingResult = null;
        });
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Added to receipt')));
      }
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _locking = false);
    }
  }

  void _cancelPending() {
    setState(() {
      _pendingProduct = null;
      _pendingResult = null;
    });
  }

  void _changePieceQuantity(int delta) {
    setState(() => _pieceQuantity = (_pieceQuantity + delta).clamp(1, 999));
  }

  void _setPieceQuantity(int quantity) {
    setState(() => _pieceQuantity = quantity.clamp(1, 999));
  }

  Future<void> _confirmPieceLine() async {
    final product = _pendingPieceProduct;
    if (product == null) return;

    setState(() {
      _locking = true;
      _error = null;
    });
    try {
      await context.read<ReceiptState>().addPieceLine(
        productId: product.id,
        quantity: _pieceQuantity,
        unitPriceCents: product.unitPriceCents,
        totalPriceCents: _pieceQuantity * product.unitPriceCents,
      );
      if (mounted) {
        setState(() => _pendingPieceProduct = null);
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Added to receipt')));
      }
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _locking = false);
    }
  }

  void _cancelPendingPiece() {
    setState(() => _pendingPieceProduct = null);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('Sell — ${widget.scale.id}')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Search products',
                prefixIcon: const Icon(Icons.search),
                suffixIcon: _query.isEmpty
                    ? null
                    : IconButton(
                        icon: const Icon(Icons.clear),
                        onPressed: () => _searchController.clear(),
                      ),
                isDense: true,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(24),
                ),
              ),
            ),
          ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.all(8),
              child: Text(_error!, style: const TextStyle(color: Colors.red)),
            ),
          if (_weighing) const LinearProgressIndicator(),
          if (_pendingResult != null && _pendingProduct != null)
            _VerifyCard(
              product: _pendingProduct!,
              result: _pendingResult!,
              locking: _locking,
              onConfirm: _confirmAndLockIn,
              onCancel: _cancelPending,
            ),
          if (_pendingPieceProduct != null)
            _QuantityCard(
              product: _pendingPieceProduct!,
              quantity: _pieceQuantity,
              locking: _locking,
              onChangeQuantity: _changePieceQuantity,
              onSetQuantity: _setPieceQuantity,
              onConfirm: _confirmPieceLine,
              onCancel: _cancelPendingPiece,
            ),
          Expanded(
            child: FutureBuilder<List<Product>>(
              future: _productsFuture,
              builder: (context, snapshot) {
                if (snapshot.connectionState != ConnectionState.done) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (snapshot.hasError) {
                  return Center(
                    child: Text('Failed to load products: ${snapshot.error}'),
                  );
                }
                final products = snapshot.data!.where((p) {
                  if (_query.isEmpty) return true;
                  return p.name.toLowerCase().contains(_query.toLowerCase());
                }).toList();
                if (products.isEmpty) {
                  return Center(child: Text('No products match "$_query".'));
                }
                return ListView.builder(
                  itemCount: products.length,
                  itemBuilder: (context, i) {
                    final product = products[i];
                    return ListTile(
                      title: Text(product.name),
                      subtitle: Text(
                        product.isPerPiece
                            ? '${ScaleTransaction.formatCents(product.unitPriceCents)} each'
                            : '${ScaleTransaction.formatCents(product.unitPriceCents)}/kg',
                      ),
                      onTap: _weighing ? null : () => _pickProduct(product),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

/// Shows what the scale itself already computed and displayed to the
/// customer, so the vendor can double-check it before it's locked in. This
/// screen never recomputes the price — it only reads back and displays
/// what the certified scale already produced.
class _VerifyCard extends StatelessWidget {
  final Product product;
  final ScaleWeighResult result;
  final bool locking;
  final VoidCallback onConfirm;
  final VoidCallback onCancel;

  const _VerifyCard({
    required this.product,
    required this.result,
    required this.locking,
    required this.onConfirm,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) {
    final weightKg = result.weightGrams / 1000;
    return Card(
      margin: const EdgeInsets.all(12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(product.name, style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 8),
            Text('Weight: ${weightKg.toStringAsFixed(3)} kg'),
            Text(
              'Unit price: ${ScaleTransaction.formatCents(product.unitPriceCents)}/kg',
            ),
            Text(
              'Total: ${ScaleTransaction.formatCents(result.priceCents)}',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: locking ? null : onCancel,
                    child: const Text('Cancel'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: FilledButton(
                    onPressed: locking ? null : onConfirm,
                    child: locking
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Confirm & add to receipt'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// A per-piece product's quantity picker: no scale is involved, so the
/// vendor just counts items and the app computes quantity x price itself.
class _QuantityCard extends StatelessWidget {
  final Product product;
  final int quantity;
  final bool locking;
  final ValueChanged<int> onChangeQuantity;
  final ValueChanged<int> onSetQuantity;
  final VoidCallback onConfirm;
  final VoidCallback onCancel;

  const _QuantityCard({
    required this.product,
    required this.quantity,
    required this.locking,
    required this.onChangeQuantity,
    required this.onSetQuantity,
    required this.onConfirm,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) {
    final total = quantity * product.unitPriceCents;
    return Card(
      margin: const EdgeInsets.all(12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(product.name, style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 12),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                IconButton.outlined(
                  onPressed: locking ? null : () => onChangeQuantity(-1),
                  icon: const Icon(Icons.remove),
                ),
                SizedBox(
                  width: 64,
                  child: TextField(
                    key: ValueKey(quantity),
                    controller: TextEditingController(
                      text: quantity.toString(),
                    ),
                    textAlign: TextAlign.center,
                    keyboardType: TextInputType.number,
                    style: Theme.of(context).textTheme.headlineSmall,
                    decoration: const InputDecoration(border: InputBorder.none),
                    onSubmitted: (value) {
                      final n = int.tryParse(value);
                      if (n != null) onSetQuantity(n);
                    },
                  ),
                ),
                IconButton.outlined(
                  onPressed: locking ? null : () => onChangeQuantity(1),
                  icon: const Icon(Icons.add),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(
              'Price each: ${ScaleTransaction.formatCents(product.unitPriceCents)}',
            ),
            Text(
              'Total: ${ScaleTransaction.formatCents(total)}',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: locking ? null : onCancel,
                    child: const Text('Cancel'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: FilledButton(
                    onPressed: locking ? null : onConfirm,
                    child: locking
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Add to receipt'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
