import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'api/core_api_client.dart';
import 'api/scale_gateway_client.dart';
import 'auth/auth_service.dart';
import 'auth/auth_state.dart';
import 'config.dart';
import 'screens/home_shell.dart';
import 'screens/login_screen.dart';
import 'state/receipt_state.dart';

void main() {
  runApp(const ScaleApp());
}

class ScaleApp extends StatelessWidget {
  const ScaleApp({super.key});

  @override
  Widget build(BuildContext context) {
    final authService = AuthService(
      AuthConfig(
        issuer: AppConfig.rauthyIssuer,
        clientId: AppConfig.rauthyClientId,
        redirectUri: AppConfig.rauthyRedirectUri,
        postLogoutRedirectUri: kIsWeb
            ? AppConfig.rauthyWebPostLogoutRedirectUri
            : null,
      ),
    );
    return ChangeNotifierProvider(
      create: (_) => AuthState(authService),
      child: MaterialApp(
        title: 'scale-app',
        theme: ThemeData(colorSchemeSeed: Colors.green, useMaterial3: true),
        home: const _RootScreen(),
      ),
    );
  }
}

/// Decides what to show: a splash while restoring a session, the login
/// screen, or the logged-in app shell. There's one login flow for every
/// user — admin vs. vendor is a role read from GET /me after login, not a
/// separate sign-in path.
class _RootScreen extends StatefulWidget {
  const _RootScreen();

  @override
  State<_RootScreen> createState() => _RootScreenState();
}

class _RootScreenState extends State<_RootScreen> {
  bool _restoring = true;

  CoreApiClient _buildApiClient() {
    final authState = context.read<AuthState>();
    return CoreApiClient(
      baseUrl: AppConfig.coreApiBaseUrl,
      authToken: () => authState.ensureFreshAccessToken(),
    );
  }

  @override
  void initState() {
    super.initState();
    _restore();
  }

  Future<void> _restore() async {
    final authState = context.read<AuthState>();
    final restored = await authState.tryRestoreSession();
    if (restored) {
      try {
        final me = await _buildApiClient().getMe();
        authState.setCurrentUser(me);
      } catch (_) {
        // The restored session isn't good enough to reach core-api (e.g.
        // the local user record was deleted) - fall back to a fresh login.
        await authState.logout();
      }
    }
    if (mounted) setState(() => _restoring = false);
  }

  @override
  Widget build(BuildContext context) {
    if (_restoring) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    final authState = context.watch<AuthState>();
    if (!authState.isLoggedIn) {
      return LoginScreen(
        buildApiClient: _buildApiClient,
        onLoggedIn: () => setState(() {}),
      );
    }

    final coreApiClient = _buildApiClient();
    return ChangeNotifierProvider(
      create: (_) => ReceiptState(coreApiClient),
      child: HomeShell(
        gatewayClient: ScaleGatewayClient(
          baseUrl: AppConfig.scaleGatewayBaseUrl,
        ),
        coreApiClient: coreApiClient,
      ),
    );
  }
}
