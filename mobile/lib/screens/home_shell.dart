import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api/core_api_client.dart';
import '../api/scale_gateway_client.dart';
import '../auth/auth_state.dart';
import '../models/scale_status.dart';
import 'admin_users_screen.dart';
import 'receipt_screen.dart';
import 'scales_screen.dart';
import 'sell_screen.dart';

/// Post-login shell: Scales and Receipt tabs for every user, plus a
/// Vendors (admin user management) tab only for admins.
class HomeShell extends StatefulWidget {
  final ScaleGatewayClient gatewayClient;
  final CoreApiClient coreApiClient;

  const HomeShell({
    super.key,
    required this.gatewayClient,
    required this.coreApiClient,
  });

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;

  void _openSellScreen(ScaleStatus scale, String holderId, String holderName) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => SellScreen(
          scale: scale,
          gatewayClient: widget.gatewayClient,
          coreApiClient: widget.coreApiClient,
          holderId: holderId,
          holderName: holderName,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final user = context.watch<AuthState>().currentUser;
    if (user == null) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    final isAdmin = user.isAdmin;

    final tabs = <Widget>[
      ScalesScreen(
        client: widget.gatewayClient,
        currentUserId: user.id,
        currentUserName: user.displayName,
        onSelectScale: (scale) =>
            _openSellScreen(scale, user.id, user.displayName),
      ),
      const ReceiptScreen(),
      if (isAdmin) AdminUsersScreen(client: widget.coreApiClient),
    ];
    final destinations = <NavigationDestination>[
      const NavigationDestination(icon: Icon(Icons.scale), label: 'Scales'),
      const NavigationDestination(
        icon: Icon(Icons.receipt_long),
        label: 'Receipt',
      ),
      if (isAdmin)
        const NavigationDestination(
          icon: Icon(Icons.admin_panel_settings),
          label: 'Vendors',
        ),
    ];

    final index = _index >= tabs.length ? 0 : _index;

    return Scaffold(
      body: IndexedStack(index: index, children: tabs),
      bottomNavigationBar: NavigationBar(
        selectedIndex: index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: destinations,
      ),
    );
  }
}
