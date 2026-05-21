import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../auth/auth_state.dart';
import '../capture/capture_screen.dart';
import '../capture/upload_service.dart';
import '../job/job_repository.dart';
import 'compose_controller.dart';
import 'style_presets.dart';

class ComposeScreen extends ConsumerStatefulWidget {
  const ComposeScreen({super.key, required this.capture});
  final CaptureResult capture;
  @override
  ConsumerState<ComposeScreen> createState() => _ComposeScreenState();
}

class _ComposeScreenState extends ConsumerState<ComposeScreen> {
  GenMode _mode = GenMode.realistic;
  StylePreset _preset = StylePreset.none;
  final _prompt = TextEditingController();
  bool _busy = false;
  String? _error;

  Future<void> _generate() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final api = ref.read(apiClientProvider);
      final c = ComposeController(
        UploadService(api.dio),
        JobRepository.fromDio(api.dio),
      );
      final r = await c.submit(
        file: widget.capture.file,
        mode: _mode,
        preset: _preset,
        prompt: _prompt.text,
      );
      if (r.errorMessage != null) {
        setState(() => _error = r.errorMessage);
        return;
      }
      if (!mounted) return;
      context.go('/job/${r.jobId}');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Design')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: Image.file(
                File(widget.capture.file.path),
                fit: BoxFit.cover,
                height: 220,
              ),
            ),
            const SizedBox(height: 24),
            Text('Mode', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            SegmentedButton<GenMode>(
              segments: const [
                ButtonSegment(value: GenMode.realistic, label: Text('Realistic')),
                ButtonSegment(
                  value: GenMode.inspirational,
                  label: Text('Inspirational'),
                ),
              ],
              selected: {_mode},
              onSelectionChanged: (s) => setState(() => _mode = s.first),
            ),
            const SizedBox(height: 8),
            Text(
              _mode.description,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 20),
            Text('Style preset',
                style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              children: StylePreset.values.map((p) {
                final selected = p == _preset;
                return ChoiceChip(
                  label: Text(p.label),
                  selected: selected,
                  onSelected: (_) => setState(() => _preset = p),
                );
              }).toList(),
            ),
            const SizedBox(height: 20),
            Text('Describe the vibe',
                style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            TextField(
              controller: _prompt,
              maxLines: 4,
              maxLength: 500,
              decoration: const InputDecoration(
                hintText: 'Light oak, hanging plants, soft afternoon light…',
              ),
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!,
                  style: TextStyle(
                      color: Theme.of(context).colorScheme.error)),
            ],
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _busy ? null : _generate,
              child: _busy
                  ? const SizedBox(
                      height: 18,
                      width: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Generate'),
            ),
          ],
        ),
      ),
    );
  }
}
