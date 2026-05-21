import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ai_image/core/api/api_client.dart';

class _StubTokenProvider implements TokenProvider {
  _StubTokenProvider(this.token);
  String? token;
  @override
  Future<String?> currentToken() async => token;
}

class _CaptureAdapter implements HttpClientAdapter {
  _CaptureAdapter(this.onRequest);
  final void Function(RequestOptions) onRequest;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    onRequest(options);
    throw DioException(requestOptions: options, message: 'stop');
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('attaches Bearer token when provider returns one', () async {
    final provider = _StubTokenProvider('test-jwt');
    final client = ApiClient(
      baseUrl: 'https://api.test',
      tokenProvider: provider,
    );
    String? seenAuth;
    client.dio.httpClientAdapter = _CaptureAdapter((opts) {
      seenAuth = opts.headers['Authorization'] as String?;
    });
    try {
      await client.dio.get<dynamic>('/anything');
    } catch (_) {
      // _CaptureAdapter throws; we only need the captured header
    }
    expect(seenAuth, 'Bearer test-jwt');
  });

  test('omits Authorization when token is null', () async {
    final client = ApiClient(
      baseUrl: 'https://api.test',
      tokenProvider: _StubTokenProvider(null),
    );
    String? seenAuth;
    client.dio.httpClientAdapter = _CaptureAdapter((opts) {
      seenAuth = opts.headers['Authorization'] as String?;
    });
    try {
      await client.dio.get<dynamic>('/anything');
    } catch (_) {}
    expect(seenAuth, isNull);
  });
}
