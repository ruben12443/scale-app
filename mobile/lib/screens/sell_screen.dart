import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api/core_api_client.dart';
import '../api/scale_gateway_client.dart';
import '../models/product.dart';
import '../models/scale_status.dart';
import '../models/transaction.dart';
import '../state/receipt_state.dart';

/// The core sell flow for one scale: pick a product, send its price/kg to
/// the scale, the scale weighs and computes the total on its own certified
/// display, then the vendor verifies that result here before locking it
/// into the current receipt with a tap.
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
  ScaleWeighResult? _pendingResult;
  Product? _pendingProduct;
  bool _weighing = false;
  bool _locking = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _productsFuture = widget.coreApiClient.listProducts();
  }

  Future<void> _pickProduct(Product product) async {
    setState(() {
      _weighing = true;
      _error = null;
    });
    try {
      final result = await widget.gatewayClient.sendPrice(
        widget.scale.id,
        product.pricePerKgCents,
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
      await context.read<ReceiptState>().addLine(
        productId: product.id,
        scaleId: widget.scale.id,
        weightGrams: result.weightGrams,
        unitPriceCents: product.pricePerKgCents,
        totalPriceCents: result.priceCents,
        scaleStatusCode: result.statusCode,
      );
      if (mounted) {
        setState(() {
          _pendingProduct = null;
          _pendingResult = null;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Added to receipt')),
        );
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('Sell — ${widget.scale.id}')),
      body: Column(
        children: [
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
          Expanded(
            child: FutureBuilder<List<Product>>(
              future: _productsFuture,
              builder: (context, snapshot) {
                if (snapshot.connectionState != ConnectionState.done) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (snapshot.hasError) {
                  return Center(child: Text('Failed to load products: ${snapshot.error}'));
                }
                final products = snapshot.data!;
                return ListView.builder(
                  itemCount: products.length,
                  itemBuilder: (context, i) {
                    final product = products[i];
                    return ListTile(
                      title: Text(product.name),
                      subtitle: Text(
                        '${ScaleTransaction.formatCents(product.pricePerKgCents)}/kg',
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
            Text('Unit price: ${ScaleTransaction.formatCents(product.pricePerKgCents)}/kg'),
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
