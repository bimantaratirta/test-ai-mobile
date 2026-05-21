import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import '../../config/env.dart';
import '../../core/api/api_client.dart';
import 'auth_repository.dart';

class _SupabaseTokenProvider implements TokenProvider {
  @override
  Future<String?> currentToken() async {
    return Supabase.instance.client.auth.currentSession?.accessToken;
  }
}

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: Env.backendUrl,
    tokenProvider: _SupabaseTokenProvider(),
  );
});

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  final api = ref.watch(apiClientProvider);
  return AuthRepository(dio: api.dio);
});
