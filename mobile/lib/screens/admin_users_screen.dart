import 'package:flutter/material.dart';

import '../api/core_api_client.dart';
import '../models/user.dart';

/// Admin-only: add or remove vendor users for this tenant. Only shown to
/// users whose GET /me role is "admin" (see home_shell.dart) — the backend
/// enforces this independently via RequireRole, this is just UI gating.
class AdminUsersScreen extends StatefulWidget {
  final CoreApiClient client;

  const AdminUsersScreen({super.key, required this.client});

  @override
  State<AdminUsersScreen> createState() => _AdminUsersScreenState();
}

class _AdminUsersScreenState extends State<AdminUsersScreen> {
  late Future<List<AppUser>> _future;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _future = widget.client.listUsers();
  }

  void _refresh() {
    setState(() => _future = widget.client.listUsers());
  }

  Future<void> _addUser() async {
    final result = await showDialog<({String email, String displayName})>(
      context: context,
      builder: (context) => const _AddUserDialog(),
    );
    if (result == null) return;
    setState(() => _busy = true);
    try {
      await widget.client.createUser(
        email: result.email,
        displayName: result.displayName,
      );
      _refresh();
    } catch (e) {
      _showError(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _deleteUser(AppUser user) async {
    setState(() => _busy = true);
    try {
      await widget.client.deleteUser(user.id);
      _refresh();
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
      appBar: AppBar(
        title: const Text('Vendors'),
        actions: [
          IconButton(
            icon: const Icon(Icons.person_add_outlined),
            onPressed: _busy ? null : _addUser,
          ),
        ],
      ),
      body: FutureBuilder<List<AppUser>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Text('Failed to load users: ${snapshot.error}'),
            );
          }
          final users = snapshot.data!;
          return ListView.builder(
            itemCount: users.length,
            itemBuilder: (context, i) {
              final user = users[i];
              return ListTile(
                title: Text(user.displayName),
                subtitle: Text('${user.email} • ${user.role}'),
                trailing: IconButton(
                  icon: const Icon(Icons.delete_outline),
                  onPressed: _busy ? null : () => _deleteUser(user),
                ),
              );
            },
          );
        },
      ),
    );
  }
}

class _AddUserDialog extends StatefulWidget {
  const _AddUserDialog();

  @override
  State<_AddUserDialog> createState() => _AddUserDialogState();
}

class _AddUserDialogState extends State<_AddUserDialog> {
  final _emailController = TextEditingController();
  final _nameController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Add vendor'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: _nameController,
            decoration: const InputDecoration(labelText: 'Display name'),
          ),
          TextField(
            controller: _emailController,
            keyboardType: TextInputType.emailAddress,
            decoration: const InputDecoration(labelText: 'Email'),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop((
            email: _emailController.text.trim(),
            displayName: _nameController.text.trim(),
          )),
          child: const Text('Add'),
        ),
      ],
    );
  }
}
