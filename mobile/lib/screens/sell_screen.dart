import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api/api_exception.dart';
import '../api/core_api_client.dart';
import '../api/scale_gateway_client.dart';
import '../models/product.dart';
import '../models/scale_status.dart';
import '../models/transaction.dart';
import '../state/receipt_state.dart';

/// How long the screen can go untouched before it's assumed abandoned and
/// bounces back to the Scales tab, releasing the claim on this scale so
/// another vendor isn't blocked by someone who walked away.
const sellScreenInactivityTimeout = Duration(seconds: 7);

/// How often the claim on this screen's scale is renewed while it's open,
/// well inside scale-gateway's own claimTTL backstop.
const sellScreenClaimRenewalInterval = Duration(seconds: 5);

/// The core sell flow for one scale. A per-kg product sends its price to
/// the scale, which weighs and computes the total on its own certified
/// display, verified here before locking it in. A per-piece product skips
/// the scale entirely: pick a quantity, and the app computes the ordinary
/// quantity x price total itself (no legal-metrology concern, since
/// nothing is physically measured).
///
/// This screen holds an exclusive claim on [scale] for as long as it's open
/// (see `POST /scales/{id}/claim`), so no other vendor can start selling
/// against the same scale at the same time. That claim is given up —
/// releasing the scale and returning to the Scales tab — whenever the
/// vendor is done or has stopped paying attention: after adding a line to
/// the receipt, after [sellScreenInactivityTimeout] with no touch on the
/// screen, or as soon as the app is backgrounded (including the phone's
/// screen locking).
class SellScreen extends StatefulWidget {
  final ScaleStatus scale;
  final ScaleGatewayClient gatewayClient;
  final CoreApiClient coreApiClient;
  final String holderId;
  final String holderName;

  const SellScreen({
    super.key,
    required this.scale,
    required this.gatewayClient,
    required this.coreApiClient,
    required this.holderId,
    required this.holderName,
  });

  @override
  State<SellScreen> createState() => _SellScreenState();
}

class _SellScreenState extends State<SellScreen>
    with WidgetsBindingObserver, SingleTickerProviderStateMixin {
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

  // Drives both the visible countdown bar under the app bar and the
  // inactivity timeout itself: elapsed fraction 0 -> 1 over
  // sellScreenInactivityTimeout, reaching 1 (AnimationStatus.completed)
  // is what triggers leaving the screen.
  late final AnimationController _inactivityController;
  Timer? _renewalTimer;
  bool _leaving = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _productsFuture = widget.coreApiClient.listProducts();
    _searchController.addListener(() {
      setState(() => _query = _searchController.text);
    });
    _inactivityController =
        AnimationController(vsync: this, duration: sellScreenInactivityTimeout)
          ..addStatusListener((status) {
            if (status == AnimationStatus.completed) _leaveScreen();
          });
    _resetInactivityTimer();
    _renewalTimer = Timer.periodic(
      sellScreenClaimRenewalInterval,
      (_) => _renewClaim(),
    );
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _inactivityController.dispose();
    _renewalTimer?.cancel();
    if (!_leaving) {
      unawaited(
        widget.gatewayClient.releaseScale(
          widget.scale.id,
          holderId: widget.holderId,
        ),
      );
    }
    _searchController.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.inactive:
      case AppLifecycleState.hidden:
        // The app is backgrounded or the screen has locked: give up the
        // claim right away rather than waiting for it to expire, so another
        // vendor isn't blocked by a phone sitting in someone's pocket.
        _inactivityController.stop();
        _renewalTimer?.cancel();
        unawaited(
          widget.gatewayClient.releaseScale(
            widget.scale.id,
            holderId: widget.holderId,
          ),
        );
      case AppLifecycleState.resumed:
        // Coming back from the background (including unlocking the phone)
        // always lands back on the Scales tab rather than resuming mid-sale
        // on a scale that may since have been reassigned.
        _leaveScreen();
      case AppLifecycleState.detached:
        break;
    }
  }

  void _resetInactivityTimer() {
    _inactivityController
      ..stop()
      ..value = 0
      ..forward();
  }

  Future<void> _renewClaim() async {
    try {
      await widget.gatewayClient.claimScale(
        widget.scale.id,
        holderId: widget.holderId,
        holderName: widget.holderName,
      );
    } on ApiException catch (e) {
      // Someone else ended up holding the scale (e.g. our claim lapsed
      // during a network hiccup and another vendor claimed it first) — stop
      // pretending we still have exclusive use of it.
      if (e.statusCode == 409) _leaveScreen();
    } catch (_) {
      // Transient failure; the next periodic renewal will retry.
    }
  }

  /// Releases the claim and returns to the Scales tab. Used for every "stop
  /// using this scale" path: a line was added, inactivity timed out, the
  /// screen locked, or the claim was lost to another vendor.
  void _leaveScreen() {
    if (_leaving || !mounted) return;
    _leaving = true;
    _inactivityController.stop();
    _renewalTimer?.cancel();
    unawaited(
      widget.gatewayClient.releaseScale(
        widget.scale.id,
        holderId: widget.holderId,
      ),
    );
    Navigator.of(context).maybePop();
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
        holderId: widget.holderId,
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
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Added to receipt')));
        // Jump back to the Scales tab so the vendor's next tap picks a
        // (possibly different) scale rather than staying parked here.
        _leaveScreen();
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
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Added to receipt')));
        _leaveScreen();
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
    // Any touch anywhere on the screen counts as activity and pushes the
    // inactivity timeout back out, regardless of which widget handles it.
    return Listener(
      onPointerDown: (_) => _resetInactivityTimer(),
      child: Scaffold(
        appBar: AppBar(title: Text('Sell — ${widget.scale.id}')),
        body: Column(
          children: [
            // Depletes over sellScreenInactivityTimeout; reaching empty is
            // what actually triggers _leaveScreen (see the status listener
            // on _inactivityController), so this is a real countdown, not
            // just decoration.
            AnimatedBuilder(
              animation: _inactivityController,
              builder: (context, _) => LinearProgressIndicator(
                value: 1 - _inactivityController.value,
                minHeight: 3,
                backgroundColor: Colors.transparent,
              ),
            ),
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
