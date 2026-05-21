import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import '../../core/realtime/jobs_channel.dart';
import '../auth/auth_state.dart';
import 'job_repository.dart';

final jobRepositoryProvider = Provider<JobRepository>((ref) {
  final api = ref.watch(apiClientProvider);
  return JobRepository.fromDio(api.dio);
});

final jobsChannelProvider = Provider<JobsChannel>((ref) {
  return JobsChannel(Supabase.instance.client);
});

/// Combines an initial REST fetch with the realtime update stream so
/// the UI can show a job immediately and then update on each change.
final jobWatchProvider =
    StreamProvider.family.autoDispose<Job, String>((ref, jobId) async* {
  final repo = ref.watch(jobRepositoryProvider);
  final channel = ref.watch(jobsChannelProvider);

  final initial = await repo.get(jobId);
  yield initial;
  if (initial.isTerminal) return;

  await for (final job in channel.watch(jobId)) {
    yield job;
    if (job.isTerminal) return;
  }
});
