import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class WelcomeScreen extends StatelessWidget {
  const WelcomeScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context).textTheme;
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Spacer(),
              Text('AI Image', style: t.displaySmall),
              const SizedBox(height: 8),
              Text(
                'Photograph an empty room. Describe the coffee shop you imagine. We design it.',
                style: t.bodyLarge,
              ),
              const Spacer(),
              FilledButton(
                onPressed: () => context.go('/login'),
                child: const Text('Get started'),
              ),
              const SizedBox(height: 12),
              Text(
                'You\'ll need an invite code (sent over WhatsApp / email).',
                style: t.bodySmall,
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
