import 'package:flutter/material.dart';

import '../api/api_exception.dart';
import '../api/scale_gateway_client.dart';
import '../models/scale_status.dart';

/// Connection status and every scale available on the local network,
/// talking to the scale-gateway service (see
/// backend/services/scale-gateway/README.md). Tapping a connected, free
/// scale claims it (see `POST /scales/{id}/claim`) and starts the sell flow
/// for it; a scale another vendor already holds shows who's using it instead
/// of being tappable, so two vendors can never end up on the same scale at
/// once. A disconnected scale should be rare, so its troubleshooting hint
/// stays collapsed until tapped.
class ScalesScreen extends StatefulWidget {
  final ScaleGatewayClient client;
  final String currentUserId;
  final String currentUserName;
  final void Function(ScaleStatus scale) onSelectScale;

  const ScalesScreen({
    super.key,
    required this.client,
    required this.currentUserId,
    required this.currentUserName,
    required this.onSelectScale,
  });

  @override
  State<ScalesScreen> createState() => _ScalesScreenState();
}

class _ScalesScreenState extends State<ScalesScreen> {
  late Future<List<ScaleStatus>> _future;
  final Set<String> _expanded = {};
  final Set<String> _retrying = {};
  String? _claiming;

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

  Future<void> _selectScale(ScaleStatus scale) async {
    setState(() => _claiming = scale.id);
    try {
      await widget.client.claimScale(
        scale.id,
        holderId: widget.currentUserId,
        holderName: widget.currentUserName,
      );
      if (!mounted) return;
      widget.onSelectScale(scale);
    } on ApiException catch (e) {
      if (!mounted) return;
      final message = e.statusCode == 409
          ? '${scale.id} is already in use'
          : 'Could not claim ${scale.id}: ${e.message}';
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(message)));
      _refresh();
    } finally {
      if (mounted) setState(() => _claiming = null);
    }
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
                final heldByOther = scale.heldByOther(widget.currentUserId);
                final claiming = _claiming == scale.id;
                return ListTile(
                  leading: Icon(
                    Icons.circle,
                    size: 12,
                    color: heldByOther ? Colors.orange : Colors.green,
                  ),
                  title: Text(scale.id),
                  subtitle: Text(
                    heldByOther
                        ? 'In use by ${scale.heldByName ?? scale.heldById}'
                        : 'Connected',
                  ),
                  trailing: claiming
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : (heldByOther ? const Icon(Icons.lock_outline) : null),
                  onTap: heldByOther || claiming
                      ? null
                      : () => _selectScale(scale),
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
