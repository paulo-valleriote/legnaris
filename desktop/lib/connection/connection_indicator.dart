import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/connection/connection_provider.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// Compact connection state for the header's top-right slot (CONN-06,
/// CONN-08): an 8px dot plus a localized status label. Every string comes
/// from the localization layer and every color from the theme's
/// [ColorScheme] — checking → onSurfaceVariant, online → primary,
/// offline → error (CONN-11).
class ConnectionIndicator extends ConsumerWidget {
  const ConnectionIndicator({super.key = const Key('connection-indicator')});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(connectionProvider).status;
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final scheme = theme.colorScheme;

    final label = switch (status) {
      ConnectionStatus.checking => localizations.connectionChecking,
      ConnectionStatus.online => localizations.connectionOnline,
      ConnectionStatus.offline => localizations.connectionOffline,
    };
    final color = switch (status) {
      ConnectionStatus.checking => scheme.onSurfaceVariant,
      ConnectionStatus.online => scheme.primary,
      ConnectionStatus.offline => scheme.error,
    };

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: AppSpacing.sm,
          height: AppSpacing.sm,
          decoration: BoxDecoration(color: color, shape: BoxShape.circle),
        ),
        const SizedBox(width: AppSpacing.xs),
        Text(label, style: theme.textTheme.bodySmall?.copyWith(color: color)),
      ],
    );
  }
}
