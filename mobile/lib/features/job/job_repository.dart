import 'package:dio/dio.dart';

import '../../core/api/api_endpoints.dart';

class Job {
  Job({
    required this.id,
    required this.mode,
    required this.status,
    required this.prompt,
    required this.inputImageUrl,
    this.stylePreset,
    this.outputImageUrl,
    this.errorMessage,
    this.generationMs,
    required this.createdAt,
    this.completedAt,
  });

  final String id;
  final String mode;
  final String status;
  final String prompt;
  final String? stylePreset;
  final String inputImageUrl;
  final String? outputImageUrl;
  final String? errorMessage;
  final int? generationMs;
  final DateTime createdAt;
  final DateTime? completedAt;

  bool get isTerminal => status == 'completed' || status == 'failed';
  bool get isCompleted => status == 'completed';
  bool get isFailed => status == 'failed';

  factory Job.fromJson(Map<String, dynamic> j) => Job(
        id: j['id'] as String,
        mode: j['mode'] as String,
        status: j['status'] as String,
        prompt: j['prompt'] as String,
        stylePreset: j['style_preset'] as String?,
        inputImageUrl: j['input_image_url'] as String,
        outputImageUrl: j['output_image_url'] as String?,
        errorMessage: j['error_message'] as String?,
        generationMs: j['generation_ms'] as int?,
        createdAt: DateTime.parse(j['created_at'] as String),
        completedAt: j['completed_at'] == null
            ? null
            : DateTime.parse(j['completed_at'] as String),
      );
}

class JobRepository {
  JobRepository.fromDio(this._dio);
  final Dio _dio;

  Future<String> create({
    required String inputImageUrl,
    required String mode,
    required String prompt,
    required String stylePreset,
  }) async {
    final resp = await _dio.post<dynamic>(
      ApiEndpoints.generate,
      data: {
        'input_image_url': inputImageUrl,
        'mode': mode,
        'prompt': prompt,
        'style_preset': stylePreset,
      },
    );
    return (resp.data as Map<String, dynamic>)['job_id'] as String;
  }

  Future<Job> get(String id) async {
    final resp = await _dio.get<dynamic>(ApiEndpoints.job(id));
    return Job.fromJson(resp.data as Map<String, dynamic>);
  }

  Future<List<Job>> list({int limit = 20, DateTime? cursor}) async {
    final resp = await _dio.get<dynamic>(
      ApiEndpoints.jobs,
      queryParameters: {
        'limit': limit,
        if (cursor != null) 'cursor': cursor.toUtc().toIso8601String(),
      },
    );
    final body = resp.data as Map<String, dynamic>;
    final raw = body['jobs'] as List<dynamic>;
    return raw
        .map((e) => Job.fromJson(e as Map<String, dynamic>))
        .toList(growable: false);
  }
}
