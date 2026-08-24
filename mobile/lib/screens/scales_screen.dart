import 'package:flutter/material.dart';

import '../api/scale_gateway_client.dart';
import '../models/scale_status.dart';

/// Connection status and every scale available on the local network,
/// talking to the scale-gateway service (see
/// backend/services/scale-gateway/README.md). Tapping a connected scale
/// starts the sell flow for it. A disconnected scale should be rare, so
/// its troubleshooting hint stays collapsed until tapped.
class ScalesScreen extends StatefulWidget {
  final ScaleGatewayClient client;
  final void Function(ScaleStatus scale) onSelectScale;

  const ScalesScreen({
    super.key,
    required this.client,
    required this.onSelectScale,
  });

  @override
  State<ScalesScreen> createState() => _ScalesScreenState();
}

class _ScalesScreenState extends State<ScalesScreen> {
  late Future<List<ScaleStatus>> _future;
  final Set<String> _expanded = {};
  final Set<String> _retrying = {};

  @override
  void initState() {
    super.initState();
    _future = widget.client.listScales();
  }

  void _refresh() {
    setState(() => _future = widget.client.listScales());
  }

  void _toggleHint(String scaleId) {
    setState(() {
      if (!_expanded.remove(scaleId)) _expanded.add(scaleId);
    });
  }

  Future<void> _retry(String scaleId) async {
    setState(() => _retrying.add(scaleId));
    try {
      final scales = await widget.client.listScales();
      if (!mounted) return;
      setState(() {
        _future = Future.value(scales);
        final stillDisconnected = scales.any(
          (s) => s.id == scaleId && !s.connected,
        );
        if (!stillDisconnected) _expanded.remove(scaleId);
      });
    } finally {
      if (mounted) setState(() => _retrying.remove(scaleId));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Scales'),
        actions: [
          IconButton(icon: const Icon(Icons.refresh), onPressed: _refresh),
        ],
      ),
      body: FutureBuilder<List<ScaleStatus>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Text('Failed to load scales: ${snapshot.error}'),
            );
          }
          final scales = snapshot.data!;
          if (scales.isEmpty) {
            return const Center(child: Text('No scales configured.'));
          }
          return ListView.builder(
            itemCount: scales.length,
            itemBuilder: (context, i) {
              final scale = scales[i];
              if (scale.connected) {
                return ListTile(
                  leading: const Icon(
                    Icons.circle,
                    size: 12,
                    color: Colors.green,
                  ),
                  title: Text(scale.id),
                  subtitle: const Text('Connected'),
                  onTap: () => widget.onSelectScale(scale),
                );
              }

              final expanded = _expanded.contains(scale.id);
              final retrying = _retrying.contains(scale.id);
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  ListTile(
                    leading: const Icon(
                      Icons.circle,
                      size: 12,
                      color: Colors.red,
                    ),
                    title: Text(scale.id),
                    subtitle: Text(scale.lastError ?? 'Not connected'),
                    trailing: Icon(
                      expanded ? Icons.expand_less : Icons.expand_more,
                    ),
                    onTap: () => _toggleHint(scale.id),
                  ),
                  if (expanded)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text(
                            'Not seeing this scale? Check that it\'s '
                            'powered on and on the same network as '
                            'scale-gateway, then retry. This should be '
                            'rare.',
                            style: TextStyle(
                              color: Colors.black54,
                              fontSize: 13,
                            ),
                          ),
                          const SizedBox(height: 8),
                          OutlinedButton(
                            onPressed: retrying ? null : () => _retry(scale.id),
                            child: retrying
                                ? const SizedBox(
                                    width: 16,
                                    height: 16,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Text('Retry connection'),
                          ),
                        ],
                      ),
                    ),
                ],
              );
            },
          );
        },
      ),
    );
  }
}
