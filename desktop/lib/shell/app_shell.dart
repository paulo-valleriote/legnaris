import 'package:flutter/material.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/left_nav.dart';
import 'package:legnaris/shell/right_panel.dart';
import 'package:legnaris/theme/tokens.dart';

/// The shell built exactly once for the whole app (CAT-01): header, hairline,
/// then the layout row — persistent left nav, width-constrained content, and
/// a right panel that hides first when the window narrows (CAT-03). The nav
/// is a fixed row child, so nothing can ever overlap it; hiding the panel is
/// pure layout, which is what keeps a resize from reloading the catalog.
class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.content});

  /// Screen mounted in the content area.
  final Widget content;

  @override
  Widget build(BuildContext context) {
    return Column(
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
    );
  }
}

/// Top bar of the shell (CAT-05): the localized app title plus the reserved
/// connection-indicator slot the connection spec fills in a later phase.
class Header extends StatelessWidget {
  const Header({super.key});

  @override
  Widget build(BuildContext context) {
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
            ),
          ],
        ),
      ),
    );
  }
}
