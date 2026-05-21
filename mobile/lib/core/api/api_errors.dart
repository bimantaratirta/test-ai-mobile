class ApiException implements Exception {
  ApiException(this.statusCode, this.message, {this.body});
  final int statusCode;
  final String message;
  final Object? body;

  bool get isRateLimited => statusCode == 429;
  bool get isUnauthorized => statusCode == 401;
  bool get isCostCap => statusCode == 503;

  @override
  String toString() => 'ApiException($statusCode): $message';
}
