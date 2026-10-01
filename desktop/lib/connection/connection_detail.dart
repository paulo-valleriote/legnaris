import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:legnaris/connection/connection_provider.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// Connection depth view for the right panel's reserved slot (CONN-09,
/// CONN-11): the current state as an 8px dot plus localized label, then the
/// timestamp of the last successful check — "never" before the first success.
/// Styles are explicit on both lines so they never fall back to the 48px
/// error style (UAT T8); colors come from the theme's [ColorScheme] only.
class ConnectionDetail extends ConsumerWidget {
  const ConnectionDetail({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(connectionProvider);
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final scheme = theme.colorScheme;

    final label = switch (state.status) {
      ConnectionStatus.checking => localizations.connectionChecking,
      ConnectionStatus.online => localizations.connectionOnline,
      ConnectionStatus.offline => localizations.connectionOffline,
    };
    final color = switch (state.status) {
      ConnectionStatus.checking => scheme.onSurfaceVariant,
      ConnectionStatus.online => scheme.primary,
      ConnectionStatus.offline => scheme.error,
    };
    final lastCheck =
        state.lastSuccess == null
            ? localizations.connectionLastCheckNever
            : localizations.connectionLastCheck(
              DateFormat.Hm().format(state.lastSuccess!),
            );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: AppSpacing.sm,
              height: AppSpacing.sm,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            ),
            const SizedBox(width: AppSpacing.xs),
            Text(
              label,
              style: theme.textTheme.bodyMedium?.copyWith(color: color),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.xs),
        Text(lastCheck, style: theme.textTheme.bodyMedium),
      ],
    );
  }
}
