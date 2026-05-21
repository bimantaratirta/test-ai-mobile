import 'package:dio/dio.dart';
import 'api_errors.dart';

abstract class TokenProvider {
  Future<String?> currentToken();
}

class ApiClient {
  ApiClient({required String baseUrl, required TokenProvider tokenProvider})
      : dio = Dio(BaseOptions(
          baseUrl: baseUrl,
          connectTimeout: const Duration(seconds: 10),
          sendTimeout: const Duration(seconds: 30),
          receiveTimeout: const Duration(seconds: 30),
          headers: {'Content-Type': 'application/json'},
        )) {
    dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        final token = await tokenProvider.currentToken();
        if (token != null && token.isNotEmpty) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        handler.next(options);
      },
      onError: (err, handler) {
        final response = err.response;
        if (response != null) {
          throw ApiException(
            response.statusCode ?? 0,
            (response.data is Map &&
                    (response.data as Map)['error'] is String)
                ? (response.data as Map)['error'] as String
                : 'http error',
            body: response.data,
          );
        }
        handler.next(err);
      },
    ));
  }

  final Dio dio;
}
