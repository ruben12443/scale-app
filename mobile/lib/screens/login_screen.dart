import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api/core_api_client.dart';
import '../auth/auth_state.dart';

/// One login flow for every user. There's no separate "admin login" —
/// admin vs. vendor is a role on the account (see GET /me), surfaced after
/// login by which screens are shown, not by a different sign-in path.
class LoginScreen extends StatefulWidget {
  final CoreApiClient Function() buildApiClient;
  final VoidCallback onLoggedIn;

  const LoginScreen({
    super.key,
    required this.buildApiClient,
    required this.onLoggedIn,
  });

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  bool _loading = false;
  String? _error;

  Future<void> _login() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final authState = context.read<AuthState>();
    try {
      await authState.login();
      final me = await widget.buildApiClient().getMe();
      authState.setCurrentUser(me);
      widget.onLoggedIn();
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.storefront, size: 64),
              const SizedBox(height: 16),
              const Text(
                'scale-app',
                style: TextStyle(fontSize: 28, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 32),
              if (_error != null) ...[
                Text(_error!, style: const TextStyle(color: Colors.red)),
                const SizedBox(height: 16),
              ],
              FilledButton.icon(
                onPressed: _loading ? null : _login,
                icon: _loading
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.login),
                label: const Text('Log in'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
