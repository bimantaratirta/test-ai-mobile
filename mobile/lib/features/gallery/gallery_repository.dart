import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../job/job_repository.dart';
import '../job/job_state.dart';

final galleryProvider =
    FutureProvider.autoDispose<List<Job>>((ref) async {
  final repo = ref.watch(jobRepositoryProvider);
  return repo.list(limit: 30);
});
