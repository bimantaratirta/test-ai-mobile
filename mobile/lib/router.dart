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
