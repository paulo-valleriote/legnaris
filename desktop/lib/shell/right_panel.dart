import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/connection/connection_detail.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// Right panel (CAT-06): the Connection section rendering the connection
/// provider's state, the theme hairline, then the Catalog section with the
/// loaded count and the last update time (only once a load has landed). The
/// panel issues no request of its own — both sections render provider state.
class RightPanel extends ConsumerWidget {
  const RightPanel({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final localizations = AppLocalizations.of(context)!;
    final state = ref.watch(catalogProvider);
    final sectionStyle = theme.textTheme.bodySmall?.copyWith(
      color: scheme.onSurfaceVariant,
    );

    return Container(
      width: AppSizes.rightPanelWidth,
      decoration: BoxDecoration(
        border: Border(
          left: BorderSide(
            color: scheme.outlineVariant,
            width: AppSizes.hairline,
          ),
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(localizations.sectionConnection, style: sectionStyle),
            const SizedBox(height: AppSpacing.sm),
            const SizedBox(
              key: Key('connection-panel-slot'),
              child: ConnectionDetail(),
            ),
            const SizedBox(height: AppSpacing.lg),
            const Divider(),
            const SizedBox(height: AppSpacing.lg),
            Text(localizations.sectionCatalog, style: sectionStyle),
            const SizedBox(height: AppSpacing.sm),
            Text(
              localizations.catalogCount(state.templates?.length ?? 0),
              style: theme.textTheme.bodyMedium,
            ),
            if (state.lastUpdated != null) ...[
              const SizedBox(height: AppSpacing.xs),
              Text(
                localizations.catalogLastUpdated(
                  DateFormat.Hm().format(state.lastUpdated!),
                ),
                style: theme.textTheme.bodyMedium,
              ),
            ],
          ],
        ),
      ),
    );
  }
}
