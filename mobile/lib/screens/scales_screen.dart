import 'package:flutter/material.dart';

import '../api/scale_gateway_client.dart';
import '../models/scale_status.dart';

/// Connection status and every scale available on the local network,
/// talking to the scale-gateway service (see
/// backend/services/scale-gateway/README.md). Tapping a connected scale
/// starts the sell flow for it.
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

  @override
  void initState() {
    super.initState();
    _future = widget.client.listScales();
  }

  void _refresh() {
    setState(() => _future = widget.client.listScales());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Scales'),
        actions: [IconButton(icon: const Icon(Icons.refresh), onPressed: _refresh)],
      ),
      body: FutureBuilder<List<ScaleStatus>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text('Failed to load scales: ${snapshot.error}'));
          }
          final scales = snapshot.data!;
          if (scales.isEmpty) {
            return const Center(child: Text('No scales configured.'));
          }
          return ListView.builder(
            itemCount: scales.length,
            itemBuilder: (context, i) {
              final scale = scales[i];
              return ListTile(
                leading: Icon(
                  Icons.circle,
                  size: 12,
                  color: scale.connected ? Colors.green : Colors.red,
                ),
                title: Text(scale.id),
                subtitle: Text(scale.connected ? 'Connected' : (scale.lastError ?? 'Not connected')),
                enabled: scale.connected,
                onTap: scale.connected ? () => widget.onSelectScale(scale) : null,
              );
            },
          );
        },
      ),
    );
  }
}
