import 'package:flutter_dotenv/flutter_dotenv.dart';

class Env {
  Env._();

  static String get backendUrl => _get('BACKEND_URL');
  static String get supabaseUrl => _get('SUPABASE_URL');
  static String get supabaseAnonKey => _get('SUPABASE_ANON_KEY');
  static String get sentryDsn => dotenv.env['SENTRY_DSN_MOBILE'] ?? '';

  static Future<void> load() async {
    await dotenv.load(fileName: '.env');
  }

  static String _get(String key) {
    final v = dotenv.env[key];
    if (v == null || v.isEmpty) {
      throw StateError('missing env var: $key');
    }
    return v;
  }
}
