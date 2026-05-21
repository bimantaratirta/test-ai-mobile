# Mobile App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Flutter app for the Coffee Shop Design Generator prototype: invite-gated signup via Supabase, photo capture/upload, dual-mode AI generation request, realtime job status, gallery — all talking to the live backend at https://ai.ashakita.net.

**Architecture:** Single Flutter app, riverpod for state, go_router with auth guard, dio for HTTP (with auto-injecting JWT interceptor), supabase_flutter for Auth + Realtime, S3-direct uploads via presigned PUT. Backend is the source of truth — mobile is a thin client over the existing REST + Realtime API.

**Tech Stack:** Flutter 3.24+, Dart 3.5+, riverpod 2.5+, go_router 14+, dio 5+, supabase_flutter 2.5+, image_picker, image_cropper, flutter_image_compress, cached_network_image, flutter_secure_storage, sentry_flutter.

**Spec:** `docs/superpowers/specs/2026-05-21-coffee-shop-design-generator-prototype-design.md`
**Live backend:** `https://ai.ashakita.net`

---

## Prerequisites (one-time, mostly already done)

- [ ] Flutter SDK installed (`flutter --version` should report 3.24+). If not, `brew install --cask flutter` on macOS.
- [ ] Xcode (for iOS builds). Open and accept license once: `sudo xcodebuild -license accept`.
- [ ] Android Studio + Android SDK (or Command-line Tools). `flutter doctor` should show all green.
- [ ] CocoaPods (`brew install cocoapods`) for iOS deps.
- [ ] Backend live + at least one invite code seeded in Supabase (`insert into invites (code) values ('BETA001');`).
- [ ] Note these values you'll plug into `mobile/.env`:
  - `BACKEND_URL=https://ai.ashakita.net`
  - `SUPABASE_URL=https://qzclcljyibbtdkuixarj.supabase.co`
  - `SUPABASE_ANON_KEY=sb_publishable_Bt_RrPOKIdt6Rwom5yy1sw_OyWJhFfx`
  - `SENTRY_DSN_MOBILE=` (create a separate Sentry project for Flutter, or reuse backend's)

---

## File Structure

After all tasks complete, `mobile/` looks like:

```
mobile/
├── pubspec.yaml
├── analysis_options.yaml
├── .env.example
├── .env                              (gitignored)
├── lib/
│   ├── main.dart
│   ├── app.dart
│   ├── router.dart
│   ├── config/
│   │   ├── env.dart
│   │   └── theme.dart
│   ├── core/
│   │   ├── api/
│   │   │   ├── api_client.dart
│   │   │   ├── api_errors.dart
│   │   │   └── api_endpoints.dart
│   │   ├── realtime/
│   │   │   └── jobs_channel.dart
│   │   ├── storage/
│   │   │   └── secure_storage.dart
│   │   └── obs/
│   │       └── sentry.dart
│   └── features/
│       ├── auth/
│       │   ├── auth_repository.dart
│       │   ├── auth_state.dart
│       │   ├── welcome_screen.dart
│       │   ├── login_screen.dart
│       │   └── invite_screen.dart
│       ├── home/
│       │   └── home_shell.dart
│       ├── capture/
│       │   ├── capture_screen.dart
│       │   └── upload_service.dart
│       ├── compose/
│       │   ├── compose_screen.dart
│       │   ├── style_presets.dart
│       │   └── compose_controller.dart
│       ├── job/
│       │   ├── job_repository.dart
│       │   ├── job_state.dart
│       │   └── job_progress_screen.dart
│       └── gallery/
│           ├── gallery_repository.dart
│           ├── gallery_screen.dart
│           └── gallery_detail_screen.dart
├── test/
│   ├── core/api/api_client_test.dart
│   ├── features/auth/auth_repository_test.dart
│   ├── features/compose/compose_controller_test.dart
│   └── features/job/job_repository_test.dart
├── ios/Runner/Info.plist                (modified for permissions)
└── android/app/src/main/AndroidManifest.xml  (modified for permissions)
```

---

## Task 1: Initialize Flutter project

**Files:**
- Create: entire `mobile/` Flutter project skeleton (delete the existing placeholder `.gitkeep`).

- [ ] **Step 1: Remove placeholder and run flutter create**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
rm -f mobile/.gitkeep
# Recreate the mobile directory as a fresh Flutter project. Pass
# --org to set the iOS bundle id / Android applicationId.
flutter create \
  --project-name ai_image \
  --org net.ashakita.aiimage \
  --platforms ios,android \
  --description "Coffee Shop Design Generator (prototype)" \
  mobile
```

- [ ] **Step 2: Verify it builds**

```bash
cd mobile
flutter pub get
flutter analyze
```
Expected: `No issues found!`

- [ ] **Step 3: Replace default analysis_options.yaml**

Replace `mobile/analysis_options.yaml` with:

```yaml
include: package:flutter_lints/flutter.yaml

analyzer:
  errors:
    invalid_annotation_target: ignore
  exclude:
    - "**/*.g.dart"
    - "**/*.freezed.dart"
    - "**/generated/**"

linter:
  rules:
    avoid_print: true
    prefer_single_quotes: true
    require_trailing_commas: true
    use_key_in_widget_constructors: true
```

Run `flutter analyze` again — should still be clean.

- [ ] **Step 4: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add mobile/
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: bootstrap Flutter project with bundle id net.ashakita.aiimage"
```

---

## Task 2: Add dependencies + env handling

**Files:**
- Modify: `mobile/pubspec.yaml`
- Create: `mobile/.env.example`
- Create: `mobile/lib/config/env.dart`
- Modify: `.gitignore` (at repo root) to add `mobile/.env`

- [ ] **Step 1: Update pubspec.yaml dependencies**

Replace the `dependencies:` and `dev_dependencies:` sections of `mobile/pubspec.yaml`:

```yaml
dependencies:
  flutter:
    sdk: flutter
  cupertino_icons: ^1.0.8
  # Routing + state
  go_router: ^14.6.2
  flutter_riverpod: ^2.6.1
  # HTTP + auth
  dio: ^5.7.0
  supabase_flutter: ^2.8.1
  # Secure storage + prefs
  flutter_secure_storage: ^9.2.2
  shared_preferences: ^2.3.3
  # Image
  image_picker: ^1.1.2
  image_cropper: ^8.0.2
  flutter_image_compress: ^2.3.0
  cached_network_image: ^3.4.1
  # Observability
  sentry_flutter: ^8.11.0
  # Env loading
  flutter_dotenv: ^5.2.1
  # Small utilities
  intl: ^0.19.0

dev_dependencies:
  flutter_test:
    sdk: flutter
  flutter_lints: ^5.0.0
  mocktail: ^1.0.4
```

In the `flutter:` section, add the asset for the .env file:

```yaml
flutter:
  uses-material-design: true
  assets:
    - .env
```

- [ ] **Step 2: Install**

```bash
cd mobile
flutter pub get
flutter analyze
```

- [ ] **Step 3: Create env example**

Create `mobile/.env.example`:

```bash
BACKEND_URL=https://ai.ashakita.net
SUPABASE_URL=https://qzclcljyibbtdkuixarj.supabase.co
SUPABASE_ANON_KEY=sb_publishable_Bt_RrPOKIdt6Rwom5yy1sw_OyWJhFfx
SENTRY_DSN_MOBILE=
```

And copy to `.env` for local use:

```bash
cp mobile/.env.example mobile/.env
# Edit mobile/.env and paste the real values
```

- [ ] **Step 4: Add mobile/.env to gitignore**

Append to `/Users/bimantara/Dev/ASHA/Pathon/ai-image/.gitignore`:

```
# Mobile env (do not commit)
mobile/.env
```

- [ ] **Step 5: Create env config**

Create `mobile/lib/config/env.dart`:

```dart
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
```

- [ ] **Step 6: Verify analyze**

```bash
cd mobile
flutter analyze
```
Expected: clean.

- [ ] **Step 7: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add mobile/pubspec.yaml mobile/pubspec.lock mobile/.env.example mobile/lib/config/env.dart .gitignore
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add deps + .env loader (backend URL, supabase, sentry)"
```

---

## Task 3: Theme + main app skeleton

**Files:**
- Create: `mobile/lib/config/theme.dart`
- Replace: `mobile/lib/main.dart`
- Create: `mobile/lib/app.dart`

- [ ] **Step 1: Theme**

Create `mobile/lib/config/theme.dart`:

```dart
import 'package:flutter/material.dart';

ThemeData buildLightTheme() {
  const seed = Color(0xFF6B4F3A); // coffee brown
  final scheme = ColorScheme.fromSeed(
    seedColor: seed,
    brightness: Brightness.light,
  );
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: scheme.surface,
    appBarTheme: AppBarTheme(
      backgroundColor: scheme.surface,
      foregroundColor: scheme.onSurface,
      centerTitle: true,
      elevation: 0,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size.fromHeight(52),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(14),
        ),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: scheme.surfaceContainerHighest,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide.none,
      ),
      contentPadding: const EdgeInsets.symmetric(
        horizontal: 16,
        vertical: 14,
      ),
    ),
  );
}
```

- [ ] **Step 2: main.dart entry**

Replace `mobile/lib/main.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import 'app.dart';
import 'config/env.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Env.load();
  await Supabase.initialize(
    url: Env.supabaseUrl,
    anonKey: Env.supabaseAnonKey,
  );
  runApp(const ProviderScope(child: App()));
}
```

- [ ] **Step 3: App widget**

Create `mobile/lib/app.dart`:

```dart
import 'package:flutter/material.dart';

import 'config/theme.dart';

class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'AI Image',
      theme: buildLightTheme(),
      home: const Scaffold(
        body: Center(child: Text('AI Image — placeholder')),
      ),
    );
  }
}
```

- [ ] **Step 4: Run + verify on simulator**

```bash
cd mobile
flutter run -d "iPhone 15"   # or any installed simulator from `flutter devices`
```

Expected: Material app opens with the placeholder text. Hot-quit with `q`.

- [ ] **Step 5: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add mobile/lib/
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add coffee-brown M3 theme + Supabase init + Riverpod ProviderScope"
```

---

## Task 4: API client (dio + JWT interceptor)

**Files:**
- Create: `mobile/lib/core/api/api_client.dart`
- Create: `mobile/lib/core/api/api_errors.dart`
- Create: `mobile/lib/core/api/api_endpoints.dart`
- Create: `mobile/test/core/api/api_client_test.dart`

- [ ] **Step 1: Endpoints**

Create `mobile/lib/core/api/api_endpoints.dart`:

```dart
class ApiEndpoints {
  ApiEndpoints._();
  static const redeemInvite = '/redeem-invite';
  static const uploadsPresign = '/uploads/presign';
  static const generate = '/generate';
  static const jobs = '/jobs';
  static String job(String id) => '/jobs/$id';
}
```

- [ ] **Step 2: Errors**

Create `mobile/lib/core/api/api_errors.dart`:

```dart
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
```

- [ ] **Step 3: Test the auth interceptor before implementing**

Create `mobile/test/core/api/api_client_test.dart`:

```dart
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ai_image/core/api/api_client.dart';

class _StubTokenProvider implements TokenProvider {
  _StubTokenProvider(this.token);
  String? token;
  @override
  Future<String?> currentToken() async => token;
}

void main() {
  test('attaches Bearer token when provider returns one', () async {
    final provider = _StubTokenProvider('test-jwt');
    final client = ApiClient(
      baseUrl: 'https://api.test',
      tokenProvider: provider,
    );
    // Capture the Authorization header by stubbing the dio adapter.
    String? seenAuth;
    client.dio.httpClientAdapter = _CaptureAdapter((opts) {
      seenAuth = opts.headers['Authorization'] as String?;
    });
    try {
      await client.dio.get<dynamic>('/anything');
    } catch (_) {/* adapter throws; we only need the captured header */}
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
```

(Note: this file needs the `import 'dart:typed_data';` for `Uint8List` — add it at the top of the test file.)

Add to imports of the test file:
```dart
import 'dart:typed_data';
```

- [ ] **Step 4: Run test (expect compile fail — ApiClient/TokenProvider not defined)**

```bash
cd mobile
flutter test test/core/api/api_client_test.dart
```
Expected: compile failure referencing `ApiClient` / `TokenProvider`.

- [ ] **Step 5: Implement the client**

Create `mobile/lib/core/api/api_client.dart`:

```dart
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
```

- [ ] **Step 6: Run test (expect pass)**

```bash
flutter test test/core/api/api_client_test.dart
```
Expected: both tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add mobile/lib/core/api/ mobile/test/core/api/
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add dio API client with JWT interceptor + ApiException"
```

---

## Task 5: Secure storage wrapper + Sentry init

**Files:**
- Create: `mobile/lib/core/storage/secure_storage.dart`
- Create: `mobile/lib/core/obs/sentry.dart`
- Modify: `mobile/lib/main.dart`

- [ ] **Step 1: Secure storage**

Create `mobile/lib/core/storage/secure_storage.dart`:

```dart
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SecureStorage {
  SecureStorage()
      : _box = const FlutterSecureStorage(
          aOptions: AndroidOptions(encryptedSharedPreferences: true),
          iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
        );

  final FlutterSecureStorage _box;

  Future<String?> read(String key) => _box.read(key: key);
  Future<void> write(String key, String value) =>
      _box.write(key: key, value: value);
  Future<void> delete(String key) => _box.delete(key: key);
  Future<void> deleteAll() => _box.deleteAll();
}
```

- [ ] **Step 2: Sentry init**

Create `mobile/lib/core/obs/sentry.dart`:

```dart
import 'package:flutter/widgets.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

import '../../config/env.dart';

Future<void> initSentryAndRun(Widget app) async {
  final dsn = Env.sentryDsn;
  if (dsn.isEmpty) {
    runApp(app);
    return;
  }
  await SentryFlutter.init(
    (options) {
      options.dsn = dsn;
      options.tracesSampleRate = 0.1;
      options.environment = 'prototype';
    },
    appRunner: () => runApp(app),
  );
}
```

- [ ] **Step 3: Wire into main.dart**

Replace `mobile/lib/main.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import 'app.dart';
import 'config/env.dart';
import 'core/obs/sentry.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Env.load();
  await Supabase.initialize(
    url: Env.supabaseUrl,
    anonKey: Env.supabaseAnonKey,
  );
  await initSentryAndRun(const ProviderScope(child: App()));
}
```

- [ ] **Step 4: Verify**

```bash
cd mobile
flutter analyze
flutter test
```
Both clean.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/core/storage mobile/lib/core/obs mobile/lib/main.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add secure storage wrapper and Sentry init"
```

---

## Task 6: Router skeleton (go_router with auth guard)

**Files:**
- Create: `mobile/lib/router.dart`
- Modify: `mobile/lib/app.dart`

- [ ] **Step 1: Build the router**

Create `mobile/lib/router.dart`:

```dart
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

// All screens are referenced by name only here. Implementations come
// in later tasks. Until those screens exist we use a placeholder
// "soon" screen so the router compiles and the app boots.

class _SoonScreen extends StatelessWidget {
  const _SoonScreen(this.label);
  final String label;
  @override
  Widget build(BuildContext context) =>
      Scaffold(body: Center(child: Text('$label — coming soon')));
}

/// Bridges a Stream into a ChangeNotifier so GoRouter can refresh on auth
/// changes. Fires notifyListeners() on every stream event (and once at
/// construction so the initial redirect runs).
class _GoRouterRefreshStream<T> extends ChangeNotifier {
  _GoRouterRefreshStream(Stream<T> stream) {
    notifyListeners();
    _sub = stream.asBroadcastStream().listen((_) => notifyListeners());
  }
  late final StreamSubscription<T> _sub;
  @override
  void dispose() {
    _sub.cancel();
    super.dispose();
  }
}

final routerProvider = Provider<GoRouter>((ref) {
  final refresh = _GoRouterRefreshStream(
    Supabase.instance.client.auth.onAuthStateChange,
  );
  ref.onDispose(refresh.dispose);

  String? sessionUserId() =>
      Supabase.instance.client.auth.currentSession?.user.id;

  return GoRouter(
    initialLocation: '/welcome',
    refreshListenable: refresh,
    redirect: (context, state) {
      final isAuthed = sessionUserId() != null;
      // Only /welcome and /login are "unauthed-only". /invite is allowed
      // for both states because the user must be signed in BEFORE they
      // can redeem (backend needs the JWT), so /login pushes them to
      // /invite after sign-in.
      final atUnauthedRoute = state.matchedLocation == '/welcome' ||
          state.matchedLocation == '/login';
      if (!isAuthed && !atUnauthedRoute) return '/welcome';
      if (isAuthed && atUnauthedRoute) return '/invite';
      return null;
    },
    routes: [
      GoRoute(path: '/welcome', builder: (_, __) => const _SoonScreen('Welcome')),
      GoRoute(path: '/login',   builder: (_, __) => const _SoonScreen('Login')),
      GoRoute(path: '/invite',  builder: (_, __) => const _SoonScreen('Invite')),
      GoRoute(path: '/home',    builder: (_, __) => const _SoonScreen('Home')),
      GoRoute(path: '/capture', builder: (_, __) => const _SoonScreen('Capture')),
      GoRoute(path: '/compose', builder: (_, __) => const _SoonScreen('Compose')),
      GoRoute(path: '/job/:id', builder: (_, st) =>
          _SoonScreen('Job ${st.pathParameters['id']}')),
      GoRoute(path: '/gallery', builder: (_, __) => const _SoonScreen('Gallery')),
      GoRoute(path: '/gallery/:id', builder: (_, st) =>
          _SoonScreen('Gallery item ${st.pathParameters['id']}')),
    ],
  );
});
```

- [ ] **Step 2: Wire router into App**

Replace `mobile/lib/app.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'config/theme.dart';
import 'router.dart';

class App extends ConsumerWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    return MaterialApp.router(
      title: 'AI Image',
      theme: buildLightTheme(),
      routerConfig: router,
    );
  }
}
```

- [ ] **Step 3: Run & verify**

```bash
cd mobile
flutter run -d "iPhone 15"
```
Expected: app opens on "Welcome — coming soon" screen (because no session exists). Hot-quit with `q`.

- [ ] **Step 4: Commit**

```bash
git add mobile/lib/router.dart mobile/lib/app.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add go_router with auth-aware redirect"
```

---

## Task 7: Auth repository + Supabase wiring

**Files:**
- Create: `mobile/lib/features/auth/auth_repository.dart`
- Create: `mobile/lib/features/auth/auth_state.dart`
- Create: `mobile/test/features/auth/auth_repository_test.dart`

- [ ] **Step 1: Write repository test (mocks Supabase Auth + dio)**

Create `mobile/test/features/auth/auth_repository_test.dart`:

```dart
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:ai_image/features/auth/auth_repository.dart';

class _MockDio extends Mock implements Dio {}

class _FakeResponse extends Mock implements Response<dynamic> {}

void main() {
  setUpAll(() {
    registerFallbackValue(Uri());
  });

  test('redeemInvite POSTs the code and returns true on 200', () async {
    final dio = _MockDio();
    final resp = Response<dynamic>(
      requestOptions: RequestOptions(path: '/redeem-invite'),
      data: {'redeemed': true},
      statusCode: 200,
    );
    when(() => dio.post<dynamic>(
          '/redeem-invite',
          data: any<dynamic>(named: 'data'),
        )).thenAnswer((_) async => resp);

    final repo = AuthRepository(dio: dio);
    final ok = await repo.redeemInvite('BETA001');
    expect(ok, isTrue);
    verify(() => dio.post<dynamic>(
          '/redeem-invite',
          data: {'code': 'BETA001'},
        )).called(1);
  });

  test('redeemInvite throws when status not 200', () async {
    final dio = _MockDio();
    when(() => dio.post<dynamic>(
          any(),
          data: any<dynamic>(named: 'data'),
        )).thenThrow(DioException(
      requestOptions: RequestOptions(path: '/redeem-invite'),
      response: Response<dynamic>(
        requestOptions: RequestOptions(path: '/redeem-invite'),
        data: {'error': 'invite code not found'},
        statusCode: 400,
      ),
    ));

    final repo = AuthRepository(dio: dio);
    await expectLater(
      () => repo.redeemInvite('NOPE'),
      throwsA(isA<Exception>()),
    );
  });
}
```

- [ ] **Step 2: Run test (expect compile fail)**

```bash
cd mobile
flutter test test/features/auth/auth_repository_test.dart
```

- [ ] **Step 3: Implement repository**

Create `mobile/lib/features/auth/auth_repository.dart`:

```dart
import 'package:dio/dio.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import '../../core/api/api_endpoints.dart';

class AuthRepository {
  AuthRepository({required Dio dio, SupabaseClient? supabase})
      : _dio = dio,
        _sb = supabase ?? Supabase.instance.client;

  final Dio _dio;
  final SupabaseClient _sb;

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
```

- [ ] **Step 4: Run test (expect pass)**

```bash
flutter test test/features/auth/auth_repository_test.dart
```

- [ ] **Step 5: Build providers (Riverpod glue)**

Create `mobile/lib/features/auth/auth_state.dart`:

```dart
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
```

- [ ] **Step 6: Verify analyze + test**

```bash
flutter analyze
flutter test
```

- [ ] **Step 7: Commit**

```bash
git add mobile/lib/features/auth mobile/test/features/auth
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add auth repository (Supabase + redeem-invite) and providers"
```

---

## Task 8: Welcome, Login, Invite screens

**Files:**
- Create: `mobile/lib/features/auth/welcome_screen.dart`
- Create: `mobile/lib/features/auth/login_screen.dart`
- Create: `mobile/lib/features/auth/invite_screen.dart`
- Modify: `mobile/lib/router.dart` (replace `_SoonScreen` for welcome/login/invite with real screens)

- [ ] **Step 1: Welcome screen**

Create `mobile/lib/features/auth/welcome_screen.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class WelcomeScreen extends StatelessWidget {
  const WelcomeScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context).textTheme;
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Spacer(),
              Text('AI Image', style: t.displaySmall),
              const SizedBox(height: 8),
              Text(
                'Photograph an empty room. Describe the coffee shop you imagine. We design it.',
                style: t.bodyLarge,
              ),
              const Spacer(),
              FilledButton(
                onPressed: () => context.go('/login'),
                child: const Text('Get started'),
              ),
              const SizedBox(height: 12),
              Text(
                'You\'ll need an invite code (sent over WhatsApp / email).',
                style: t.bodySmall,
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 2: Login screen**

Create `mobile/lib/features/auth/login_screen.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'auth_state.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});
  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _email = TextEditingController();
  final _password = TextEditingController();
  bool _isSignUp = false;
  bool _busy = false;
  String? _error;

  Future<void> _submit() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final auth = ref.read(authRepositoryProvider);
      if (_isSignUp) {
        await auth.signUpEmail(_email.text.trim(), _password.text);
      } else {
        await auth.signInEmail(_email.text.trim(), _password.text);
      }
      if (!mounted) return;
      // After sign in, route to invite redemption if profile doesn't exist
      // yet. For MVP we just send everyone through /invite first; users
      // already redeemed will see "already used" but can proceed.
      context.go('/invite');
    } on Exception catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(_isSignUp ? 'Sign up' : 'Sign in')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                controller: _email,
                keyboardType: TextInputType.emailAddress,
                autocorrect: false,
                decoration: const InputDecoration(hintText: 'Email'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _password,
                obscureText: true,
                decoration: const InputDecoration(hintText: 'Password'),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!,
                    style: TextStyle(
                        color: Theme.of(context).colorScheme.error)),
              ],
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _busy ? null : _submit,
                child: _busy
                    ? const SizedBox(
                        height: 18,
                        width: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(_isSignUp ? 'Create account' : 'Sign in'),
              ),
              TextButton(
                onPressed: () => setState(() => _isSignUp = !_isSignUp),
                child: Text(_isSignUp
                    ? 'Have an account? Sign in'
                    : 'New here? Create account'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 3: Invite screen**

Create `mobile/lib/features/auth/invite_screen.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'auth_state.dart';

class InviteScreen extends ConsumerStatefulWidget {
  const InviteScreen({super.key});
  @override
  ConsumerState<InviteScreen> createState() => _InviteScreenState();
}

class _InviteScreenState extends ConsumerState<InviteScreen> {
  final _code = TextEditingController();
  bool _busy = false;
  String? _error;

  Future<void> _redeem() async {
    final session = ref.read(authRepositoryProvider).currentSession;
    if (session == null) {
      context.go('/login');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final ok = await ref
          .read(authRepositoryProvider)
          .redeemInvite(_code.text.trim().toUpperCase());
      if (!ok) {
        setState(() => _error = 'Invite not accepted. Try again.');
        return;
      }
      if (!mounted) return;
      context.go('/home');
    } on Exception catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Invite code')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Text(
                'Enter the invite code you received to unlock generation.',
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _code,
                textCapitalization: TextCapitalization.characters,
                decoration: const InputDecoration(hintText: 'BETA001'),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!,
                    style: TextStyle(
                        color: Theme.of(context).colorScheme.error)),
              ],
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _busy ? null : _redeem,
                child: _busy
                    ? const SizedBox(
                        height: 18,
                        width: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Redeem'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 4: Wire screens into router**

In `mobile/lib/router.dart`, replace the three `_SoonScreen(...)` entries for `/welcome`, `/login`, `/invite` with the real widgets:

```dart
// add imports at top
import 'features/auth/welcome_screen.dart';
import 'features/auth/login_screen.dart';
import 'features/auth/invite_screen.dart';

// in the routes list, replace:
GoRoute(path: '/welcome', builder: (_, __) => const WelcomeScreen()),
GoRoute(path: '/login',   builder: (_, __) => const LoginScreen()),
GoRoute(path: '/invite',  builder: (_, __) => const InviteScreen()),
```

- [ ] **Step 5: Smoke test on simulator**

```bash
cd mobile
flutter run -d "iPhone 15"
```

Manually:
1. Welcome screen appears.
2. Tap "Get started" → login screen.
3. Sign up with `test2@example.com` / `Test12345!` (or use an existing user).
4. Should land on invite screen.
5. Enter `BETA002` (one of the codes you seeded earlier).
6. Should redirect to `/home` → "Home — coming soon" (real Home in Task 9).

Quit with `q`.

- [ ] **Step 6: Commit**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git add mobile/lib/features/auth/ mobile/lib/router.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add Welcome / Login / Invite screens wired into router"
```

---

## Task 9: Home shell (bottom nav scaffold)

**Files:**
- Create: `mobile/lib/features/home/home_shell.dart`
- Modify: `mobile/lib/router.dart`

- [ ] **Step 1: Home shell**

Create `mobile/lib/features/home/home_shell.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../auth/auth_state.dart';

class HomeShell extends ConsumerStatefulWidget {
  const HomeShell({super.key});
  @override
  ConsumerState<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends ConsumerState<HomeShell> {
  int _tab = 0;

  void _onSelect(int i) {
    setState(() => _tab = i);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('AI Image'),
        actions: [
          IconButton(
            tooltip: 'Sign out',
            icon: const Icon(Icons.logout),
            onPressed: () async {
              await ref.read(authRepositoryProvider).signOut();
              if (context.mounted) context.go('/welcome');
            },
          ),
        ],
      ),
      body: IndexedStack(
        index: _tab,
        children: const [
          _ComposeTabCTA(),
          _GalleryTabPlaceholder(),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: _onSelect,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.add_a_photo_outlined),
            selectedIcon: Icon(Icons.add_a_photo),
            label: 'New',
          ),
          NavigationDestination(
            icon: Icon(Icons.collections_outlined),
            selectedIcon: Icon(Icons.collections),
            label: 'Gallery',
          ),
        ],
      ),
    );
  }
}

class _ComposeTabCTA extends StatelessWidget {
  const _ComposeTabCTA();
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.add_a_photo_outlined,
              size: 96,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 24),
            Text(
              'Start with an empty room',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            Text(
              'Photograph the space, describe the vision.',
              style: Theme.of(context).textTheme.bodyMedium,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 32),
            FilledButton.icon(
              icon: const Icon(Icons.add),
              label: const Text('New design'),
              onPressed: () => context.go('/compose'),
            ),
          ],
        ),
      ),
    );
  }
}

class _GalleryTabPlaceholder extends StatelessWidget {
  const _GalleryTabPlaceholder();
  @override
  Widget build(BuildContext context) {
    // Real gallery comes in Task 14. For now, redirect.
    return const Center(child: Text('Gallery — coming soon'));
  }
}
```

- [ ] **Step 2: Wire into router**

In `mobile/lib/router.dart`, replace `GoRoute(path: '/home', ...)` with:

```dart
// add import
import 'features/home/home_shell.dart';

// in routes list:
GoRoute(path: '/home', builder: (_, __) => const HomeShell()),
```

- [ ] **Step 3: Smoke test**

```bash
flutter run -d "iPhone 15"
```

Land on Home, tap "New design" → routes to `/compose` placeholder. Sign out button → returns to Welcome.

- [ ] **Step 4: Commit**

```bash
git add mobile/lib/features/home mobile/lib/router.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add Home shell with bottom nav (Compose/Gallery) + sign out"
```

---

## Task 10: Image picker / capture + upload service

**Files:**
- Create: `mobile/lib/features/capture/capture_screen.dart`
- Create: `mobile/lib/features/capture/upload_service.dart`
- Modify: `mobile/ios/Runner/Info.plist` (add NSPhotoLibrary + NSCamera usage descriptions)
- Modify: `mobile/android/app/src/main/AndroidManifest.xml` (add camera permission)

- [ ] **Step 1: Add iOS Info.plist permissions**

Open `mobile/ios/Runner/Info.plist`. Inside the top-level `<dict>` add these keys (before the closing `</dict>`):

```xml
<key>NSCameraUsageDescription</key>
<string>Used to photograph the empty room you want designed.</string>
<key>NSPhotoLibraryUsageDescription</key>
<string>Used to pick an empty-room photo from your library.</string>
<key>NSPhotoLibraryAddUsageDescription</key>
<string>Used to save generated designs to your photos.</string>
```

- [ ] **Step 2: Android manifest permissions**

In `mobile/android/app/src/main/AndroidManifest.xml`, inside `<manifest ...>` (above the `<application ...>` tag) add:

```xml
<uses-permission android:name="android.permission.CAMERA" />
<uses-permission android:name="android.permission.READ_MEDIA_IMAGES" />
```

- [ ] **Step 3: Upload service**

Create `mobile/lib/features/capture/upload_service.dart`:

```dart
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
```

- [ ] **Step 4: Capture screen**

Create `mobile/lib/features/capture/capture_screen.dart`:

```dart
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:image_cropper/image_cropper.dart';
import 'package:image_picker/image_picker.dart';
import 'package:go_router/go_router.dart';

/// Returned as a route argument to /compose so the compose screen can show
/// a preview thumbnail and submit the file path on Generate.
class CaptureResult {
  CaptureResult(this.file);
  final File file;
}

class CaptureScreen extends StatefulWidget {
  const CaptureScreen({super.key});
  @override
  State<CaptureScreen> createState() => _CaptureScreenState();
}

class _CaptureScreenState extends State<CaptureScreen> {
  final _picker = ImagePicker();
  bool _busy = false;

  Future<void> _pick(ImageSource source) async {
    setState(() => _busy = true);
    try {
      final picked = await _picker.pickImage(
        source: source,
        imageQuality: 92,
        maxWidth: 3000,
        maxHeight: 3000,
      );
      if (picked == null) return;

      final cropped = await ImageCropper().cropImage(
        sourcePath: picked.path,
        aspectRatio: const CropAspectRatio(ratioX: 4, ratioY: 3),
        uiSettings: [
          AndroidUiSettings(
            toolbarTitle: 'Crop empty room',
            lockAspectRatio: true,
          ),
          IOSUiSettings(
            title: 'Crop empty room',
            aspectRatioLockEnabled: true,
          ),
        ],
      );
      if (cropped == null) return;

      if (!mounted) return;
      context.go('/compose', extra: CaptureResult(File(cropped.path)));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Capture')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              FilledButton.icon(
                icon: const Icon(Icons.camera_alt),
                label: const Text('Take a photo'),
                onPressed: _busy ? null : () => _pick(ImageSource.camera),
              ),
              const SizedBox(height: 16),
              OutlinedButton.icon(
                icon: const Icon(Icons.photo_library),
                label: const Text('Choose from library'),
                onPressed: _busy ? null : () => _pick(ImageSource.gallery),
              ),
              if (_busy) ...[
                const SizedBox(height: 24),
                const CircularProgressIndicator(),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 5: Route /capture**

In `mobile/lib/router.dart`, ADD a new route entry (and the import):

```dart
import 'features/capture/capture_screen.dart';
// ... in routes list, add:
GoRoute(path: '/capture', builder: (_, __) => const CaptureScreen()),
```

Also change `home_shell.dart`'s "New design" button to push `/capture` instead of `/compose`:

In `mobile/lib/features/home/home_shell.dart`, replace `onPressed: () => context.go('/compose')` with `onPressed: () => context.go('/capture')`.

- [ ] **Step 6: Smoke test**

```bash
flutter run -d "iPhone 15"
```

From Home → "New design" → picks photo / camera → crops → routes to `/compose` placeholder with `CaptureResult` as extra. Verify by inspecting that `/compose` placeholder shows (we wire the real compose screen next task).

- [ ] **Step 7: Commit**

```bash
git add mobile/lib/features/capture mobile/lib/router.dart \
  mobile/lib/features/home/home_shell.dart \
  mobile/ios/Runner/Info.plist \
  mobile/android/app/src/main/AndroidManifest.xml
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add capture flow (camera+gallery+crop) and upload service"
```

---

## Task 11: Compose screen + controller

**Files:**
- Create: `mobile/lib/features/compose/style_presets.dart`
- Create: `mobile/lib/features/compose/compose_screen.dart`
- Create: `mobile/lib/features/compose/compose_controller.dart`
- Create: `mobile/test/features/compose/compose_controller_test.dart`
- Modify: `mobile/lib/router.dart` (real `/compose` route + accepts extra)

- [ ] **Step 1: Style presets enum**

Create `mobile/lib/features/compose/style_presets.dart`:

```dart
enum StylePreset { scandinavian, industrial, japandi, cozyWarm, minimalist, none }

extension StylePresetX on StylePreset {
  String get label {
    switch (this) {
      case StylePreset.scandinavian: return 'Scandinavian';
      case StylePreset.industrial:   return 'Industrial';
      case StylePreset.japandi:      return 'Japandi';
      case StylePreset.cozyWarm:     return 'Cozy Warm';
      case StylePreset.minimalist:   return 'Minimalist';
      case StylePreset.none:         return 'No preset';
    }
  }

  String get apiValue {
    switch (this) {
      case StylePreset.scandinavian: return 'scandinavian';
      case StylePreset.industrial:   return 'industrial';
      case StylePreset.japandi:      return 'japandi';
      case StylePreset.cozyWarm:     return 'cozy_warm';
      case StylePreset.minimalist:   return 'minimalist';
      case StylePreset.none:         return '';
    }
  }
}

enum GenMode { realistic, inspirational }

extension GenModeX on GenMode {
  String get label => this == GenMode.realistic ? 'Realistic' : 'Inspirational';
  String get description => this == GenMode.realistic
      ? 'Keep the room\'s structure. Add furniture, lighting, decor.'
      : 'Reimagine the space freely based on the description.';
  String get apiValue => this == GenMode.realistic ? 'realistic' : 'inspirational';
}
```

- [ ] **Step 2: Controller test**

Create `mobile/test/features/compose/compose_controller_test.dart`:

```dart
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:ai_image/features/capture/upload_service.dart';
import 'package:ai_image/features/compose/compose_controller.dart';
import 'package:ai_image/features/compose/style_presets.dart';
import 'package:ai_image/features/job/job_repository.dart';

class _MockUpload extends Mock implements UploadService {}
class _MockJobs extends Mock implements JobRepository {}

void main() {
  setUpAll(() {
    registerFallbackValue(File('x'));
  });

  test('submit: validates prompt length', () async {
    final c = ComposeController(_MockUpload(), _MockJobs());
    final r = await c.submit(
      file: File('/tmp/x.jpg'),
      mode: GenMode.realistic,
      preset: StylePreset.scandinavian,
      prompt: 'no',
    );
    expect(r.errorMessage, isNotNull);
    expect(r.jobId, isNull);
  });

  test('submit: uploads, calls /generate, returns job_id', () async {
    final upload = _MockUpload();
    final jobs = _MockJobs();

    when(() => upload.uploadAndGetPublicUrl(any())).thenAnswer(
      (_) async => 'https://cdn/x.jpg',
    );
    when(() => jobs.create(
      inputImageUrl: 'https://cdn/x.jpg',
      mode: 'realistic',
      prompt: 'Scandinavian shop with light oak',
      stylePreset: 'scandinavian',
    )).thenAnswer((_) async => 'job-123');

    final c = ComposeController(upload, jobs);
    final r = await c.submit(
      file: File('/tmp/x.jpg'),
      mode: GenMode.realistic,
      preset: StylePreset.scandinavian,
      prompt: 'Scandinavian shop with light oak',
    );
    expect(r.errorMessage, isNull);
    expect(r.jobId, 'job-123');
  });
}
```

- [ ] **Step 3: Controller**

Create `mobile/lib/features/compose/compose_controller.dart`:

```dart
import 'dart:io';

import '../capture/upload_service.dart';
import '../job/job_repository.dart';
import 'style_presets.dart';

class ComposeResult {
  ComposeResult({this.jobId, this.errorMessage});
  final String? jobId;
  final String? errorMessage;
}

class ComposeController {
  ComposeController(this._upload, this._jobs);
  final UploadService _upload;
  final JobRepository _jobs;

  Future<ComposeResult> submit({
    required File file,
    required GenMode mode,
    required StylePreset preset,
    required String prompt,
  }) async {
    final trimmed = prompt.trim();
    if (trimmed.length < 5 || trimmed.length > 500) {
      return ComposeResult(
          errorMessage: 'Prompt must be 5–500 characters.');
    }
    try {
      final url = await _upload.uploadAndGetPublicUrl(file);
      final jobId = await _jobs.create(
        inputImageUrl: url,
        mode: mode.apiValue,
        prompt: trimmed,
        stylePreset: preset.apiValue,
      );
      return ComposeResult(jobId: jobId);
    } on Exception catch (e) {
      return ComposeResult(errorMessage: e.toString());
    }
  }
}
```

(This test depends on `JobRepository` which we'll create in Task 12. For now, just stub it so the test file compiles.)

- [ ] **Step 4: Stub `JobRepository` so the test compiles**

Create the placeholder file `mobile/lib/features/job/job_repository.dart`:

```dart
class JobRepository {
  JobRepository();

  /// Returns the created job_id. Real impl in Task 12.
  Future<String> create({
    required String inputImageUrl,
    required String mode,
    required String prompt,
    required String stylePreset,
  }) async {
    throw UnimplementedError('Task 12 wires this up');
  }
}
```

- [ ] **Step 5: Run controller test**

```bash
cd mobile
flutter test test/features/compose/compose_controller_test.dart
```
Expected: both tests PASS.

- [ ] **Step 6: Compose screen**

Create `mobile/lib/features/compose/compose_screen.dart`:

```dart
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/api/api_client.dart';
import '../capture/upload_service.dart';
import '../capture/capture_screen.dart';
import '../job/job_repository.dart';
import 'compose_controller.dart';
import 'style_presets.dart';

class ComposeScreen extends ConsumerStatefulWidget {
  const ComposeScreen({super.key, required this.capture});
  final CaptureResult capture;
  @override
  ConsumerState<ComposeScreen> createState() => _ComposeScreenState();
}

class _ComposeScreenState extends ConsumerState<ComposeScreen> {
  GenMode _mode = GenMode.realistic;
  StylePreset _preset = StylePreset.none;
  final _prompt = TextEditingController();
  bool _busy = false;
  String? _error;

  Future<void> _generate() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final api = ref.read(apiClientProvider);
      final c = ComposeController(
        UploadService(api.dio),
        JobRepository.fromDio(api.dio),
      );
      final r = await c.submit(
        file: widget.capture.file,
        mode: _mode,
        preset: _preset,
        prompt: _prompt.text,
      );
      if (r.errorMessage != null) {
        setState(() => _error = r.errorMessage);
        return;
      }
      if (!mounted) return;
      context.go('/job/${r.jobId}');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Design')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: Image.file(
                File(widget.capture.file.path),
                fit: BoxFit.cover,
                height: 220,
              ),
            ),
            const SizedBox(height: 24),
            Text('Mode', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            SegmentedButton<GenMode>(
              segments: const [
                ButtonSegment(value: GenMode.realistic, label: Text('Realistic')),
                ButtonSegment(
                    value: GenMode.inspirational, label: Text('Inspirational')),
              ],
              selected: {_mode},
              onSelectionChanged: (s) => setState(() => _mode = s.first),
            ),
            const SizedBox(height: 8),
            Text(
              _mode.description,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 20),
            Text('Style preset', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              children: StylePreset.values.map((p) {
                final selected = p == _preset;
                return ChoiceChip(
                  label: Text(p.label),
                  selected: selected,
                  onSelected: (_) => setState(() => _preset = p),
                );
              }).toList(),
            ),
            const SizedBox(height: 20),
            Text('Describe the vibe', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            TextField(
              controller: _prompt,
              maxLines: 4,
              maxLength: 500,
              decoration: const InputDecoration(
                hintText: 'Light oak, hanging plants, soft afternoon light…',
              ),
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ],
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _busy ? null : _generate,
              child: _busy
                  ? const SizedBox(
                      height: 18,
                      width: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Generate'),
            ),
          ],
        ),
      ),
    );
  }
}
```

- [ ] **Step 7: Wire `/compose` route to accept extra**

In `mobile/lib/router.dart`:
- Add the import: `import 'features/compose/compose_screen.dart';` and `import 'features/capture/capture_screen.dart';`
- Replace the compose GoRoute with:

```dart
GoRoute(
  path: '/compose',
  builder: (_, st) {
    final extra = st.extra;
    if (extra is! CaptureResult) {
      // No image carried over — bounce back to /capture.
      return const Scaffold(
        body: Center(child: Text('Please pick a photo first.')),
      );
    }
    return ComposeScreen(capture: extra);
  },
),
```

- [ ] **Step 8: Commit**

```bash
flutter analyze
flutter test
git add mobile/lib/features/compose mobile/lib/features/job mobile/test/features/compose mobile/lib/router.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add compose screen with mode/preset/prompt and submission flow"
```

---

## Task 12: Job repository + realtime subscription

**Files:**
- Modify: `mobile/lib/features/job/job_repository.dart` (replace stub with real impl)
- Create: `mobile/lib/features/job/job_state.dart`
- Create: `mobile/lib/core/realtime/jobs_channel.dart`
- Create: `mobile/test/features/job/job_repository_test.dart`

- [ ] **Step 1: Job repository test**

Create `mobile/test/features/job/job_repository_test.dart`:

```dart
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
```

- [ ] **Step 2: Job repository (replace stub)**

Replace `mobile/lib/features/job/job_repository.dart`:

```dart
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
```

- [ ] **Step 3: Run tests**

```bash
cd mobile
flutter test test/features/job/
```
Expected: both tests PASS.

- [ ] **Step 4: Realtime channel**

Create `mobile/lib/core/realtime/jobs_channel.dart`:

```dart
import 'dart:async';

import 'package:supabase_flutter/supabase_flutter.dart';

import '../../features/job/job_repository.dart';

/// Subscribes to the Supabase Realtime `jobs` table for changes to a
/// specific row. Emits the updated [Job] whenever it changes.
class JobsChannel {
  JobsChannel(this._sb);
  final SupabaseClient _sb;

  /// Stream of updates for [jobId]. Closes when [cancel] is called.
  Stream<Job> watch(String jobId) {
    final controller = StreamController<Job>();
    final channel = _sb.channel('jobs:$jobId');

    channel
        .onPostgresChanges(
          event: PostgresChangeEvent.update,
          schema: 'public',
          table: 'jobs',
          filter: PostgresChangeFilter(
            type: PostgresChangeFilterType.eq,
            column: 'id',
            value: jobId,
          ),
          callback: (payload) {
            final row = payload.newRecord;
            try {
              controller.add(Job.fromJson(Map<String, dynamic>.from(row)));
            } catch (e) {
              controller.addError(e);
            }
          },
        )
        .subscribe();

    controller.onCancel = () async {
      await _sb.removeChannel(channel);
    };
    return controller.stream;
  }
}
```

- [ ] **Step 5: Job state (providers)**

Create `mobile/lib/features/job/job_state.dart`:

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:supabase_flutter/supabase_flutter.dart';

import '../../core/realtime/jobs_channel.dart';
import '../auth/auth_state.dart';
import 'job_repository.dart';

final jobRepositoryProvider = Provider<JobRepository>((ref) {
  final api = ref.watch(apiClientProvider);
  return JobRepository.fromDio(api.dio);
});

final jobsChannelProvider = Provider<JobsChannel>((ref) {
  return JobsChannel(Supabase.instance.client);
});

/// Combines an initial REST fetch with the realtime update stream so
/// the UI can show a job immediately and then update on each change.
final jobWatchProvider =
    StreamProvider.family.autoDispose<Job, String>((ref, jobId) async* {
  final repo = ref.watch(jobRepositoryProvider);
  final channel = ref.watch(jobsChannelProvider);

  final initial = await repo.get(jobId);
  yield initial;
  if (initial.isTerminal) return;

  await for (final job in channel.watch(jobId)) {
    yield job;
    if (job.isTerminal) return;
  }
});
```

- [ ] **Step 6: Commit**

```bash
flutter analyze
flutter test
git add mobile/lib/core/realtime mobile/lib/features/job mobile/test/features/job
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add job repository + realtime channel + watch provider"
```

---

## Task 13: Job progress screen

**Files:**
- Create: `mobile/lib/features/job/job_progress_screen.dart`
- Modify: `mobile/lib/router.dart` (replace placeholder `/job/:id`)

- [ ] **Step 1: Progress screen**

Create `mobile/lib/features/job/job_progress_screen.dart`:

```dart
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'job_state.dart';

class JobProgressScreen extends ConsumerWidget {
  const JobProgressScreen({super.key, required this.jobId});
  final String jobId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(jobWatchProvider(jobId));

    return Scaffold(
      appBar: AppBar(title: const Text('Generating')),
      body: SafeArea(
        child: async.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text('Error: $e')),
          data: (job) {
            if (job.isCompleted) {
              return _Done(jobId: job.id, output: job.outputImageUrl!);
            }
            if (job.isFailed) {
              return _Failed(message: job.errorMessage ?? 'Unknown error');
            }
            return _InProgress(status: job.status);
          },
        ),
      ),
    );
  }
}

class _InProgress extends StatelessWidget {
  const _InProgress({required this.status});
  final String status;
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 24),
          Text(
            status == 'queued'
                ? 'In queue…'
                : 'AI is designing your space…',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          Text(
            'Typically 10–30 seconds. You can keep the app open.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}

class _Done extends StatelessWidget {
  const _Done({required this.jobId, required this.output});
  final String jobId;
  final String output;
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: CachedNetworkImage(
                imageUrl: output,
                fit: BoxFit.cover,
                placeholder: (_, __) => const Center(
                  child: CircularProgressIndicator(),
                ),
              ),
            ),
          ),
          const SizedBox(height: 16),
          FilledButton(
            onPressed: () => context.go('/home'),
            child: const Text('Done'),
          ),
          OutlinedButton(
            onPressed: () => context.go('/gallery/$jobId'),
            child: const Text('Open in gallery'),
          ),
        ],
      ),
    );
  }
}

class _Failed extends StatelessWidget {
  const _Failed({required this.message});
  final String message;
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Icon(
            Icons.error_outline,
            size: 64,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 16),
          Text(
            'Generation failed',
            style: Theme.of(context).textTheme.titleLarge,
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 8),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: () => context.go('/home'),
            child: const Text('Back home'),
          ),
        ],
      ),
    );
  }
}
```

- [ ] **Step 2: Route**

In `mobile/lib/router.dart`:
- Add `import 'features/job/job_progress_screen.dart';`
- Replace the `/job/:id` route:

```dart
GoRoute(
  path: '/job/:id',
  builder: (_, st) =>
      JobProgressScreen(jobId: st.pathParameters['id']!),
),
```

- [ ] **Step 3: Smoke test (end-to-end with backend)**

```bash
flutter run -d "iPhone 15"
```

Manually:
1. Sign in.
2. Pick / capture an empty room photo.
3. Fill compose form (Scandinavian, "Light oak, plants").
4. Tap Generate.
5. Watch the progress screen show "In queue…" → "AI is designing…" → final result image.

- [ ] **Step 4: Commit**

```bash
git add mobile/lib/features/job/job_progress_screen.dart mobile/lib/router.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add job progress screen with realtime updates"
```

---

## Task 14: Gallery list

**Files:**
- Create: `mobile/lib/features/gallery/gallery_repository.dart`
- Create: `mobile/lib/features/gallery/gallery_screen.dart`
- Modify: `mobile/lib/features/home/home_shell.dart` (replace placeholder gallery tab)

- [ ] **Step 1: Repository (just re-exports job listing)**

Create `mobile/lib/features/gallery/gallery_repository.dart`:

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../job/job_repository.dart';
import '../job/job_state.dart';

final galleryProvider =
    FutureProvider.autoDispose<List<Job>>((ref) async {
  final repo = ref.watch(jobRepositoryProvider);
  return repo.list(limit: 30);
});
```

- [ ] **Step 2: Grid screen**

Create `mobile/lib/features/gallery/gallery_screen.dart`:

```dart
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'gallery_repository.dart';

class GalleryScreen extends ConsumerWidget {
  const GalleryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(galleryProvider);
    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(galleryProvider),
      child: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Error: $e')),
        data: (jobs) {
          if (jobs.isEmpty) {
            return ListView(
              children: const [
                SizedBox(height: 200),
                Center(child: Text('No designs yet. Generate your first!')),
              ],
            );
          }
          return GridView.builder(
            padding: const EdgeInsets.all(8),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 2,
              mainAxisSpacing: 8,
              crossAxisSpacing: 8,
              childAspectRatio: 4 / 3,
            ),
            itemCount: jobs.length,
            itemBuilder: (_, i) {
              final j = jobs[i];
              return GestureDetector(
                onTap: () => context.go('/gallery/${j.id}'),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(10),
                  child: j.outputImageUrl != null
                      ? CachedNetworkImage(
                          imageUrl: j.outputImageUrl!,
                          fit: BoxFit.cover,
                          placeholder: (_, __) => const ColoredBox(
                              color: Color(0xFFE5E0DA)),
                        )
                      : ColoredBox(
                          color: Theme.of(context)
                              .colorScheme
                              .surfaceContainerHighest,
                          child: const Center(
                            child: Text(
                              'failed',
                              style: TextStyle(color: Colors.grey),
                            ),
                          ),
                        ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}
```

- [ ] **Step 3: Wire into home shell**

In `mobile/lib/features/home/home_shell.dart`, replace `_GalleryTabPlaceholder` with the real screen:

```dart
import '../gallery/gallery_screen.dart';

// in IndexedStack children:
children: const [
  _ComposeTabCTA(),
  GalleryScreen(),
],
```

Delete the `_GalleryTabPlaceholder` class.

- [ ] **Step 4: Smoke test**

```bash
flutter run
```
After completing a generation, switch to the Gallery tab — the result thumbnail should appear.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/features/gallery mobile/lib/features/home/home_shell.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add gallery grid with cached thumbnails"
```

---

## Task 15: Gallery detail screen

**Files:**
- Create: `mobile/lib/features/gallery/gallery_detail_screen.dart`
- Modify: `mobile/lib/router.dart`

- [ ] **Step 1: Detail screen**

Create `mobile/lib/features/gallery/gallery_detail_screen.dart`:

```dart
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../job/job_state.dart';

class GalleryDetailScreen extends ConsumerWidget {
  const GalleryDetailScreen({super.key, required this.jobId});
  final String jobId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(jobWatchProvider(jobId));
    return Scaffold(
      appBar: AppBar(title: const Text('Design')),
      body: SafeArea(
        child: async.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text('Error: $e')),
          data: (job) => ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Text(
                'Before',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: CachedNetworkImage(
                  imageUrl: job.inputImageUrl,
                  fit: BoxFit.cover,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'After',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              if (job.outputImageUrl != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(12),
                  child: CachedNetworkImage(
                    imageUrl: job.outputImageUrl!,
                    fit: BoxFit.cover,
                  ),
                )
              else
                Padding(
                  padding: const EdgeInsets.all(24),
                  child: Center(
                    child: Text(job.errorMessage ?? 'No output'),
                  ),
                ),
              const SizedBox(height: 24),
              _Meta(label: 'Mode', value: job.mode),
              _Meta(label: 'Style preset', value: job.stylePreset ?? '—'),
              _Meta(label: 'Prompt', value: job.prompt),
              if (job.generationMs != null)
                _Meta(
                  label: 'Generation time',
                  value: '${(job.generationMs! / 1000).toStringAsFixed(1)}s',
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Meta extends StatelessWidget {
  const _Meta({required this.label, required this.value});
  final String label;
  final String value;
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 120,
            child: Text(label,
                style: Theme.of(context).textTheme.bodySmall),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}
```

- [ ] **Step 2: Route**

In `mobile/lib/router.dart`:
- Add `import 'features/gallery/gallery_detail_screen.dart';`
- Replace the `/gallery/:id` route:

```dart
GoRoute(
  path: '/gallery/:id',
  builder: (_, st) =>
      GalleryDetailScreen(jobId: st.pathParameters['id']!),
),
```

- [ ] **Step 3: Smoke test**

In simulator: tap a gallery thumbnail, see before/after side by side. Pinch zoom is built into the underlying image widget.

- [ ] **Step 4: Commit**

```bash
git add mobile/lib/features/gallery/gallery_detail_screen.dart mobile/lib/router.dart
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile: add gallery detail (before/after + metadata)"
```

---

## Task 16: iOS build configuration

**Files:**
- Modify: `mobile/ios/Runner/Info.plist`
- Modify: `mobile/ios/Runner.xcodeproj/project.pbxproj` (bundle id verification, only — done by `flutter create --org`)

- [ ] **Step 1: Set display name in Info.plist**

In `mobile/ios/Runner/Info.plist`, find `CFBundleDisplayName` and ensure value is "AI Image" (it defaults to "Ai Image" from project name underscores — change it):

```xml
<key>CFBundleDisplayName</key>
<string>AI Image</string>
```

Also add (inside the top-level `<dict>`):

```xml
<!-- Required by Supabase OAuth redirect handler (Apple/Google sign-in)
     and certbot-issued certs for our backend domain. -->
<key>NSAppTransportSecurity</key>
<dict>
  <key>NSAllowsArbitraryLoads</key>
  <false/>
</dict>
```

- [ ] **Step 2: Try a build**

```bash
cd mobile
flutter build ios --no-codesign
```
Expected: successful build without code-signing errors. (Real TestFlight upload requires Apple Developer Program, which we set up in a later phase / out of scope here.)

- [ ] **Step 3: Commit**

```bash
git add mobile/ios/
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile/ios: set display name, lock ATS, verify build"
```

---

## Task 17: Android build configuration

**Files:**
- Modify: `mobile/android/app/build.gradle.kts` (or `build.gradle`)
- Modify: `mobile/android/app/src/main/AndroidManifest.xml`

- [ ] **Step 1: Set minSdk / targetSdk + app label**

Open `mobile/android/app/build.gradle.kts`. In `defaultConfig`, ensure:
- `minSdk = 24` (Supabase Realtime requires modern TLS)
- `targetSdk` matches Flutter SDK default

If `build.gradle.kts` doesn't exist (older flutter create uses `build.gradle`), make analogous edits there.

- [ ] **Step 2: App label in manifest**

In `mobile/android/app/src/main/AndroidManifest.xml`, change `android:label` to "AI Image":

```xml
<application
    android:label="AI Image"
    ...>
```

- [ ] **Step 3: Try build**

```bash
flutter build apk --debug
```
Expected: successful APK output at `build/app/outputs/flutter-apk/app-debug.apk`.

- [ ] **Step 4: Commit**

```bash
git add mobile/android/
git -c user.email=198cad@gmail.com -c user.name="Bimantara" commit -m "mobile/android: set minSdk 24, app label, verify debug APK build"
```

---

## Task 18: Verify on a real device + push

**Files:** none — manual.

- [ ] **Step 1: Hook up a real device**

iOS: plug iPhone via USB, "Trust this computer" prompt → `flutter devices` shows it.

Android: enable Developer Options + USB debugging on the device → `flutter devices` shows it.

- [ ] **Step 2: Run on device**

```bash
flutter run -d <device-id-from-flutter-devices>
```

Walk through the full happy path:
1. Welcome → Sign up
2. Invite redemption with a seeded code
3. Capture (camera or library) → crop to 4:3
4. Compose: Realistic mode, Scandinavian preset, "Light oak, plants, soft window light"
5. Tap Generate → progress → result image
6. Open Gallery → see thumbnail → tap → see before/after

If anything fails, capture screenshots / Sentry events for triage.

- [ ] **Step 3: Push final commit & tag**

```bash
cd /Users/bimantara/Dev/ASHA/Pathon/ai-image
git push
git tag mobile-v0.1.0 -m "Mobile prototype: full beta flow on real devices"
git push --tags
```

---

## Done

At this point the mobile app delivers the full beta product:

- Invite-gated signup via Supabase Auth (email/password; Apple/Google can be added later when developer accounts exist).
- Photograph or pick an empty room, crop to 4:3.
- Compose with mode (Realistic / Inspirational), style preset chip, freeform prompt.
- Submit → backend uploads via presigned URL → Supabase Realtime pushes status updates → progress screen reflects them.
- Gallery shows the history of generations with before/after detail.
- Sentry captures errors silently in production builds.

Next phases (out of scope for this plan):
- TestFlight + Google Play Internal Testing distribution (need Apple Developer Program $99/yr + Google Play Developer $25 one-time).
- Apple / Google sign-in (need OAuth client IDs + redirect URLs).
- RevenueCat-driven subscription (mature version).
- Localization (Indonesian + English).
