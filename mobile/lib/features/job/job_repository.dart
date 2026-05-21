import 'package:dio/dio.dart';

/// Stub for Task 11. Replaced by full implementation in Task 12.
class JobRepository {
  JobRepository.fromDio(this._dio);
  // ignore: unused_field
  final Dio _dio;

  /// Returns the created job_id. Real impl wires POST /generate in Task 12.
  Future<String> create({
    required String inputImageUrl,
    required String mode,
    required String prompt,
    required String stylePreset,
  }) async {
    throw UnimplementedError('Task 12 wires this up');
  }
}
