import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'auth_state.dart';

class InviteScreen extends ConsumerStatefulWidget {
  const InviteScreen({super.key});
  @override
  ConsumerState<InviteScreen> createState() => _InviteScreenState();
}

class _InviteScreenState extends ConsumerState<InviteScreen> {
  final _code = TextEditingController();
  bool _busy = false;
  String? _error;

  Future<void> _redeem() async {
    final session = ref.read(authRepositoryProvider).currentSession;
    if (session == null) {
      context.go('/login');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final ok = await ref
          .read(authRepositoryProvider)
          .redeemInvite(_code.text.trim().toUpperCase());
      if (!ok) {
        setState(() => _error = 'Invite not accepted. Try again.');
        return;
      }
      if (!mounted) return;
      context.go('/home');
    } on Exception catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Invite code')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Text(
                'Enter the invite code you received to unlock generation.',
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _code,
                textCapitalization: TextCapitalization.characters,
                decoration: const InputDecoration(hintText: 'BETA001'),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!,
                    style: TextStyle(
                        color: Theme.of(context).colorScheme.error)),
              ],
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _busy ? null : _redeem,
                child: _busy
                    ? const SizedBox(
                        height: 18,
                        width: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Redeem'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
