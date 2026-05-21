import 'package:dio/dio.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import '../../core/api/api_endpoints.dart';

class AuthRepository {
  AuthRepository({required Dio dio, SupabaseClient? supabase})
      : _dio = dio,
        _sbOverride = supabase;

  final Dio _dio;
  final SupabaseClient? _sbOverride;

  SupabaseClient get _sb => _sbOverride ?? Supabase.instance.client;

  Session? get currentSession => _sb.auth.currentSession;
  User? get currentUser => _sb.auth.currentUser;

  Future<AuthResponse> signUpEmail(String email, String password) async {
    return _sb.auth.signUp(email: email, password: password);
  }

  Future<AuthResponse> signInEmail(String email, String password) async {
    return _sb.auth.signInWithPassword(email: email, password: password);
  }

  Future<bool> signInWithGoogle() async {
    return _sb.auth.signInWithOAuth(OAuthProvider.google);
  }

  Future<bool> signInWithApple() async {
    return _sb.auth.signInWithOAuth(OAuthProvider.apple);
  }

  Future<void> signOut() => _sb.auth.signOut();

  /// Calls the backend to mark the invite code as used by the current user.
  /// Returns true on success. Throws on failure.
  Future<bool> redeemInvite(String code) async {
    final resp = await _dio.post<dynamic>(
      ApiEndpoints.redeemInvite,
      data: {'code': code},
    );
    if (resp.statusCode != 200) {
      throw Exception('redeem failed: ${resp.statusCode}');
    }
    final body = resp.data;
    return body is Map && body['redeemed'] == true;
  }
}
