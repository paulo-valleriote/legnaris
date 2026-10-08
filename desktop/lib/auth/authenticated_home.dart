import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_route.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/tokens.dart';

/// The authenticated surface: resolves the user's single sheet through
/// `GET /sheets` and routes to open or create (FE6-13 … FE6-15, FE6-20).
///
/// The lookup runs exactly once per authenticated transition — from
/// `initState`, not `build` — so a rebuild never issues a second request. The
/// settled surface is always the catalog: a `200` pushes Screen 2 as a route
/// above it (NAV-05), a `404` presents the create flow, and any other failure
/// renders the mapped message with a Retry that re-issues the request and
/// never touches the session.
class AuthenticatedHome extends ConsumerStatefulWidget {
  const AuthenticatedHome({super.key});

  @override
  ConsumerState<AuthenticatedHome> createState() => _AuthenticatedHomeState();
}

class _AuthenticatedHomeState extends ConsumerState<AuthenticatedHome> {
  bool _loading = true;
  String? _errorMessage;

  /// Guards the single-flight lookup, so Retry and the mount load never
  /// overlap.
  bool _inFlight = false;

  /// True once Screen 2 was pushed for this authenticated transition: a
  /// Retry success after a failure (or any other re-run) never pushes a
  /// second route (NAV-07).
  bool _sheetPushed = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      // Reset the per-session shell surface (NAV-02): a forced logout that
      // disposed the creation dialog never runs the dialog's own restore
      // (`catalog_screen.dart` finally), so the next session must not
      // inherit the stuck surface.
      ref.read(shellStateProvider.notifier).show(ShellSurface.catalog);
      _load();
    });
  }

  /// One `GET /sheets`: a `200` pushes Screen 2 above the catalog exactly
  /// once (NAV-05, NAV-07), null (404) falls through to the catalog create
  /// flow, and any failure carries the mapped copy with Retry (FE6-13 …
  /// FE6-15, FE6-20).
  Future<void> _load() async {
    if (_inFlight) return;
    _inFlight = true;
    setState(() {
      _loading = true;
      _errorMessage = null;
    });
    try {
      final sheet = await ref.read(apiClientProvider).getMySheet();
      if (!mounted) return;
      setState(() => _loading = false);
      if (sheet != null && !_sheetPushed) {
        _sheetPushed = true;
        unawaited(pushSheetScreen(context, sheetId: sheet.id));
      }
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _errorMessage = failureCopy(error);
      });
    } finally {
      _inFlight = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final localizations = AppLocalizations.of(context)!;
    if (_loading) {
      return AppShell(content: _loadingBody(context, localizations));
    }
    if (_errorMessage != null) {
      return AppShell(
        content: _errorBody(context, _errorMessage!, localizations),
      );
    }
    return const AppShell(content: CatalogScreen());
  }

  Widget _loadingBody(BuildContext context, AppLocalizations localizations) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const LinearProgressIndicator(),
        const SizedBox(height: AppSpacing.sm),
        Text(
          localizations.sheetLoading,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  /// A non-404 failure: the mapped copy plus one Retry that re-issues the
  /// lookup, keeping the authenticated session (FE6-20).
  Widget _errorBody(
    BuildContext context,
    String message,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Expanded(
          child: Text(
            message,
            key: const Key('home-sheet-error'),
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.error,
            ),
          ),
        ),
        const SizedBox(width: AppSpacing.md),
        OutlinedButton(
          key: const Key('home-sheet-retry'),
          onPressed: _load,
          child: Text(localizations.actionRetry),
        ),
      ],
    );
  }
}
