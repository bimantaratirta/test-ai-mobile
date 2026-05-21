import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_image_compress/flutter_image_compress.dart';

import '../../core/api/api_endpoints.dart';
import '../../core/api/api_errors.dart';

class UploadService {
  UploadService(this._dio);
  final Dio _dio;

  /// Compresses [file] (max ~2MB JPG) and uploads via a presigned URL the
  /// backend hands out. Returns the public URL that the backend will see
  /// as `input_image_url` on the /generate call.
  Future<String> uploadAndGetPublicUrl(File file) async {
    final compressed = await _compressIfNeeded(file);

    final presign = await _dio.post<dynamic>(
      ApiEndpoints.uploadsPresign,
      data: {'content_type': 'image/jpeg'},
    );
    final body = presign.data as Map<String, dynamic>;
    final uploadUrl = body['upload_url'] as String;
    final publicUrl = body['public_url'] as String;

    final upload = await Dio().put<dynamic>(
      uploadUrl,
      data: compressed.openRead(),
      options: Options(
        headers: {
          'Content-Type': 'image/jpeg',
          Headers.contentLengthHeader: await compressed.length(),
        },
      ),
    );
    if (upload.statusCode != 200) {
      throw ApiException(upload.statusCode ?? 0, 'upload failed');
    }
    return publicUrl;
  }

  Future<File> _compressIfNeeded(File f) async {
    final size = await f.length();
    if (size < 1024 * 1024 * 2) return f;
    final out = '${f.parent.path}/compressed_${DateTime.now().millisecondsSinceEpoch}.jpg';
    final result = await FlutterImageCompress.compressAndGetFile(
      f.absolute.path,
      out,
      quality: 80,
      minWidth: 1600,
      minHeight: 1600,
      format: CompressFormat.jpeg,
    );
    if (result == null) return f;
    return File(result.path);
  }
}
