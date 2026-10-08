import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/connection/connection_indicator.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/left_nav.dart';
import 'package:legnaris/shell/right_panel.dart';
import 'package:legnaris/theme/tokens.dart';

/// The shell built exactly once for the whole app (CAT-01): header, hairline,
/// then the layout row — persistent left nav, width-constrained content, and
/// a right panel that hides first when the window narrows (CAT-03). The nav
/// is a fixed row child, so nothing can ever overlap it; hiding the panel is
/// pure layout, which is what keeps a resize from reloading the catalog.
///
/// The [Scaffold] is load-bearing, not decoration: it paints the surface
/// (a bare column would leave the window at its default black) and provides
/// the Material context in which an unstyled [Text] resolves to `bodyMedium`
/// — without it MaterialApp substitutes its 48px red `_errorTextStyle`
/// fallback (UAT T8).
class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.content});

  /// Screen mounted in the content area.
  final Widget content;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Header(),
          const Divider(),
          Expanded(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final showPanel =
                    constraints.maxWidth >= AppSizes.shellMinWidthWithPanel;
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const LeftNav(),
                    const VerticalDivider(),
                    Expanded(
                      child: Align(
                        alignment: Alignment.topCenter,
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(
                            maxWidth: AppSizes.contentMaxWidth,
                          ),
                          child: Padding(
                            padding: const EdgeInsets.all(AppSpacing.lg),
                            child: content,
                          ),
                        ),
                      ),
                    ),
                    if (showPanel) ...[
                      const VerticalDivider(),
                      const RightPanel(),
                    ],
                  ],
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

/// Top bar of the shell (CAT-05): the localized app title, the connection
/// indicator in its reserved slot, and the logout action that ends the
/// session (FE6-07, FE6-19).
class Header extends ConsumerWidget {
  const Header({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    return SizedBox(
      height: AppSizes.headerHeight,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg),
        child: Row(
          children: [
            Text(localizations.appTitle, style: theme.textTheme.titleLarge),
            const Spacer(),
            const SizedBox(
              width: AppSizes.connectionSlotWidth,
              key: Key('connection-indicator-slot'),
              child: Align(
                alignment: Alignment.centerRight,
                child: ConnectionIndicator(),
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            TextButton(
              key: const Key('shell-logout'),
              onPressed: () =>
                  ref.read(authSessionProvider.notifier).logout(),
              child: Text(localizations.authLogoutAction),
            ),
          ],
        ),
      ),
    );
  }
}
