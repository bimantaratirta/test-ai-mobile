import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:ai_image/features/capture/upload_service.dart';
import 'package:ai_image/features/compose/compose_controller.dart';
import 'package:ai_image/features/compose/style_presets.dart';
import 'package:ai_image/features/job/job_repository.dart';

class _MockUpload extends Mock implements UploadService {}
class _MockJobs extends Mock implements JobRepository {}

void main() {
  setUpAll(() {
    registerFallbackValue(File('x'));
  });

  test('submit: validates prompt length', () async {
    final c = ComposeController(_MockUpload(), _MockJobs());
    final r = await c.submit(
      file: File('/tmp/x.jpg'),
      mode: GenMode.realistic,
      preset: StylePreset.scandinavian,
      prompt: 'no',
    );
    expect(r.errorMessage, isNotNull);
    expect(r.jobId, isNull);
  });

  test('submit: uploads, calls /generate, returns job_id', () async {
    final upload = _MockUpload();
    final jobs = _MockJobs();

    when(() => upload.uploadAndGetPublicUrl(any())).thenAnswer(
      (_) async => 'https://cdn/x.jpg',
    );
    when(() => jobs.create(
          inputImageUrl: 'https://cdn/x.jpg',
          mode: 'realistic',
          prompt: 'Scandinavian shop with light oak',
          stylePreset: 'scandinavian',
        )).thenAnswer((_) async => 'job-123');

    final c = ComposeController(upload, jobs);
    final r = await c.submit(
      file: File('/tmp/x.jpg'),
      mode: GenMode.realistic,
      preset: StylePreset.scandinavian,
      prompt: 'Scandinavian shop with light oak',
    );
    expect(r.errorMessage, isNull);
    expect(r.jobId, 'job-123');
  });
}
