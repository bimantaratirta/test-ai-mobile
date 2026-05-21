import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'gallery_repository.dart';

class GalleryScreen extends ConsumerWidget {
  const GalleryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(galleryProvider);
    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(galleryProvider),
      child: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Error: $e')),
        data: (jobs) {
          if (jobs.isEmpty) {
            return ListView(
              children: const [
                SizedBox(height: 200),
                Center(child: Text('No designs yet. Generate your first!')),
              ],
            );
          }
          return GridView.builder(
            padding: const EdgeInsets.all(8),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 2,
              mainAxisSpacing: 8,
              crossAxisSpacing: 8,
              childAspectRatio: 4 / 3,
            ),
            itemCount: jobs.length,
            itemBuilder: (_, i) {
              final j = jobs[i];
              return GestureDetector(
                onTap: () => context.push('/gallery/${j.id}'),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(10),
                  child: j.outputImageUrl != null
                      ? CachedNetworkImage(
                          imageUrl: j.outputImageUrl!,
                          fit: BoxFit.cover,
                          placeholder: (_, __) => const ColoredBox(
                            color: Color(0xFFE5E0DA),
                          ),
                        )
                      : ColoredBox(
                          color: Theme.of(context)
                              .colorScheme
                              .surfaceContainerHighest,
                          child: const Center(
                            child: Text(
                              'failed',
                              style: TextStyle(color: Colors.grey),
                            ),
                          ),
                        ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}
