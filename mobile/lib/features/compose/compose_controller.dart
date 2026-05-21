import 'dart:io';

import '../capture/upload_service.dart';
import '../job/job_repository.dart';
import 'style_presets.dart';

class ComposeResult {
  ComposeResult({this.jobId, this.errorMessage});
  final String? jobId;
  final String? errorMessage;
}

class ComposeController {
  ComposeController(this._upload, this._jobs);
  final UploadService _upload;
  final JobRepository _jobs;

  Future<ComposeResult> submit({
    required File file,
    required GenMode mode,
    required StylePreset preset,
    required String prompt,
  }) async {
    final trimmed = prompt.trim();
    if (trimmed.length < 5 || trimmed.length > 500) {
      return ComposeResult(
        errorMessage: 'Prompt must be 5–500 characters.',
      );
    }
    try {
      final url = await _upload.uploadAndGetPublicUrl(file);
      final jobId = await _jobs.create(
        inputImageUrl: url,
        mode: mode.apiValue,
        prompt: trimmed,
        stylePreset: preset.apiValue,
      );
      return ComposeResult(jobId: jobId);
    } on Exception catch (e) {
      return ComposeResult(errorMessage: e.toString());
    }
  }
}
