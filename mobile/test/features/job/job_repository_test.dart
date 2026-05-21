import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:ai_image/features/job/job_repository.dart';

class _MockDio extends Mock implements Dio {}

void main() {
  test('create POSTs /generate and returns job_id', () async {
    final dio = _MockDio();
    when(() => dio.post<dynamic>(
          '/generate',
          data: any<dynamic>(named: 'data'),
        )).thenAnswer((_) async => Response<dynamic>(
          requestOptions: RequestOptions(path: '/generate'),
          data: {
            'job_id': 'abc-123',
            'status': 'queued',
            'estimated_seconds': 25,
          },
          statusCode: 200,
        ));

    final repo = JobRepository.fromDio(dio);
    final id = await repo.create(
      inputImageUrl: 'https://cdn/x.jpg',
      mode: 'realistic',
      prompt: 'Scandinavian shop with oak',
      stylePreset: 'scandinavian',
    );
    expect(id, 'abc-123');
  });

  test('get returns parsed Job', () async {
    final dio = _MockDio();
    when(() => dio.get<dynamic>('/jobs/abc-123')).thenAnswer(
      (_) async => Response<dynamic>(
        requestOptions: RequestOptions(path: '/jobs/abc-123'),
        data: {
          'id': 'abc-123',
          'mode': 'realistic',
          'status': 'completed',
          'prompt': 'p',
          'input_image_url': 'https://cdn/in.jpg',
          'output_image_url': 'https://cdn/out.png',
          'error_message': null,
          'generation_ms': 12000,
          'created_at': '2026-05-22T01:23:45Z',
          'completed_at': '2026-05-22T01:23:57Z',
          'style_preset': 'scandinavian',
        },
        statusCode: 200,
      ),
    );

    final repo = JobRepository.fromDio(dio);
    final job = await repo.get('abc-123');
    expect(job.id, 'abc-123');
    expect(job.status, 'completed');
    expect(job.outputImageUrl, 'https://cdn/out.png');
  });
}
