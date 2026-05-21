import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../job/job_state.dart';

class GalleryDetailScreen extends ConsumerWidget {
  const GalleryDetailScreen({super.key, required this.jobId});
  final String jobId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(jobWatchProvider(jobId));
    return Scaffold(
      appBar: AppBar(title: const Text('Design')),
      body: SafeArea(
        child: async.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text('Error: $e')),
          data: (job) => ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Text(
                'Before',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: CachedNetworkImage(
                  imageUrl: job.inputImageUrl,
                  fit: BoxFit.cover,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'After',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              if (job.outputImageUrl != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(12),
                  child: CachedNetworkImage(
                    imageUrl: job.outputImageUrl!,
                    fit: BoxFit.cover,
                  ),
                )
              else
                Padding(
                  padding: const EdgeInsets.all(24),
                  child: Center(
                    child: Text(job.errorMessage ?? 'No output'),
                  ),
                ),
              const SizedBox(height: 24),
              _Meta(label: 'Mode', value: job.mode),
              _Meta(label: 'Style preset', value: job.stylePreset ?? '—'),
              _Meta(label: 'Prompt', value: job.prompt),
              if (job.generationMs != null)
                _Meta(
                  label: 'Generation time',
                  value: '${(job.generationMs! / 1000).toStringAsFixed(1)}s',
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Meta extends StatelessWidget {
  const _Meta({required this.label, required this.value});
  final String label;
  final String value;
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 120,
            child: Text(label,
                style: Theme.of(context).textTheme.bodySmall),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}
