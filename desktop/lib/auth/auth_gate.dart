import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/auth/auth_screen.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/authenticated_home.dart';
import 'package:legnaris/connection/connection_provider.dart';

/// Decides between the auth screen and the authenticated app (FE6-12,
/// FE6-16).
///
/// It restores a stored session exactly once on first mount and shows a
/// minimal loading state until that restore settles, so a returning user never
/// sees the login form flash. The public connection poll is kept alive here so
/// `GET /health` keeps running while unauthenticated (FE6-16): the indicator
/// provider is independent of the session.
class AuthGate extends ConsumerStatefulWidget {
  const AuthGate({super.key, this.navigatorObservers = const []});

  /// Observer seam mirroring `MaterialApp.navigatorObservers`, handed to the
  /// session navigator so tests can count the routes a session pushes
  /// (NAV-07) without depending on route-dispose notifications.
  final List<NavigatorObserver> navigatorObservers;

  @override
  ConsumerState<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends ConsumerState<AuthGate> {
  /// True once the first restore settled. Until then the gate shows the
  /// loading state; afterwards a login/register in flight keeps the auth
  /// screen mounted (the session is authenticating but the screen owns the
  /// form and its fields).
  bool _restoreCompleted = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted) return;
      await ref.read(authSessionProvider.notifier).restore();
      if (!mounted) return;
      setState(() => _restoreCompleted = true);
    });
  }

  @override
  Widget build(BuildContext context) {
    // Creates and holds the connection provider for the app's lifetime, so
    // the public health poll runs even while only the auth screen is shown
    // (FE6-16). A listener, not a watch: a health tick must not rebuild the
    // gate and its subtree.
    ref.listen(connectionProvider, (_, _) {});
    final state = ref.watch(authSessionProvider);
    if (state is AuthAuthenticated) {
      // The authenticated branch owns its navigation stack (NAV-01 … NAV-04):
      // every route pushed inside lives and dies with this Navigator, and the
      // widget-type alternation with the branches below guarantees a fresh
      // NavigatorState — an empty stack — per authenticated transition
      // (NAV-02).
      return Navigator(
        observers: widget.navigatorObservers,
        onGenerateRoute: (settings) => MaterialPageRoute<void>(
          builder: (_) => const AuthenticatedHome(),
        ),
      );
    }
    if (!_restoreCompleted) return const _RestoringSession();
    return const AuthScreen();
  }
}

/// Minimal loading state shown only while the first restore is in flight.
class _RestoringSession extends StatelessWidget {
  const _RestoringSession();

  @override
  Widget build(BuildContext context) => const Scaffold(
    body: Center(
      child: CircularProgressIndicator(key: Key('auth-restoring')),
    ),
  );
}
