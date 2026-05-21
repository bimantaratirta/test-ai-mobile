import 'dart:async';

import 'package:supabase_flutter/supabase_flutter.dart';

import '../../features/job/job_repository.dart';

/// Subscribes to the Supabase Realtime `jobs` table for changes to a
/// specific row. Emits the updated [Job] whenever it changes.
class JobsChannel {
  JobsChannel(this._sb);
  final SupabaseClient _sb;

  /// Stream of updates for [jobId]. Closes when the stream is cancelled.
  Stream<Job> watch(String jobId) {
    final controller = StreamController<Job>();
    final channel = _sb.channel('jobs:$jobId');

    channel
        .onPostgresChanges(
          event: PostgresChangeEvent.update,
          schema: 'public',
          table: 'jobs',
          filter: PostgresChangeFilter(
            type: PostgresChangeFilterType.eq,
            column: 'id',
            value: jobId,
          ),
          callback: (payload) {
            final row = payload.newRecord;
            try {
              controller.add(Job.fromJson(Map<String, dynamic>.from(row)));
            } on Exception catch (e) {
              controller.addError(e);
            }
          },
        )
        .subscribe();

    controller.onCancel = () async {
      await _sb.removeChannel(channel);
    };
    return controller.stream;
  }
}
