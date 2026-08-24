import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/transaction.dart';
import '../state/receipt_state.dart';

/// The current draft receipt: mutable at any point (remove a line to
/// correct a mistake) until finalized. Once finalized it can be emailed.
class ReceiptScreen extends StatefulWidget {
  const ReceiptScreen({super.key});

  @override
  State<ReceiptScreen> createState() => _ReceiptScreenState();
}

class _ReceiptScreenState extends State<ReceiptScreen> {
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<ReceiptState>().refresh();
    });
  }

  Future<void> _removeLine(String transactionId) async {
    setState(() => _busy = true);
    try {
      await context.read<ReceiptState>().removeLine(transactionId);
    } catch (e) {
      _showError(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _finalize() async {
    setState(() => _busy = true);
    try {
      await context.read<ReceiptState>().finalizeReceipt();
    } catch (e) {
      _showError(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _emailReceipt() async {
    final receiptState = context.read<ReceiptState>();
    final to = await showDialog<String>(
      context: context,
      builder: (context) => _EmailDialog(),
    );
    if (to == null || to.isEmpty) return;
    setState(() => _busy = true);
    try {
      await receiptState.emailReceipt(to);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Receipt emailed to $to')),
        );
      }
    } catch (e) {
      _showError(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _showError(Object e) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$e')));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Receipt')),
      body: Consumer<ReceiptState>(
        builder: (context, state, _) {
          if (state.loading && state.current == null) {
            return const Center(child: CircularProgressIndicator());
          }
          final receipt = state.current;
          if (receipt == null) {
            return Center(child: Text(state.error ?? 'No receipt yet.'));
          }

          return Column(
            children: [
              Expanded(
                child: receipt.lines.isEmpty
                    ? const Center(child: Text('No items yet.'))
                    : ListView.builder(
                        itemCount: receipt.lines.length,
                        itemBuilder: (context, i) {
                          final line = receipt.lines[i];
                          return ListTile(
                            title: Text(line.productName),
                            subtitle: Text(
                              '${line.formattedWeight} • ${ScaleTransaction.formatCents(line.unitPriceCents)}/kg',
                            ),
                            trailing: receipt.isDraft
                                ? IconButton(
                                    icon: const Icon(Icons.delete_outline),
                                    onPressed: _busy ? null : () => _removeLine(line.id),
                                  )
                                : Text(ScaleTransaction.formatCents(line.totalPriceCents)),
                          );
                        },
                      ),
              ),
              Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  children: [
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        const Text('Total', style: TextStyle(fontSize: 18)),
                        Text(
                          ScaleTransaction.formatCents(receipt.totalCents),
                          style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    if (receipt.isDraft)
                      FilledButton(
                        onPressed: _busy || receipt.lines.isEmpty ? null : _finalize,
                        child: const Text('Finalize receipt'),
                      )
                    else
                      OutlinedButton.icon(
                        onPressed: _busy ? null : _emailReceipt,
                        icon: const Icon(Icons.email_outlined),
                        label: const Text('Email receipt'),
                      ),
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _EmailDialog extends StatefulWidget {
  @override
  State<_EmailDialog> createState() => _EmailDialogState();
}

class _EmailDialogState extends State<_EmailDialog> {
  final _controller = TextEditingController();

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Email receipt'),
      content: TextField(
        controller: _controller,
        keyboardType: TextInputType.emailAddress,
        decoration: const InputDecoration(hintText: 'customer@example.com'),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(_controller.text.trim()),
          child: const Text('Send'),
        ),
      ],
    );
  }
}
