import 'package:flutter/material.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// The creation surface of CAT-15/16: navigation contract only — the dialog
/// content belongs to the template-detail spec. Receives [templateId],
/// exposes it, and closes; opened with `barrierDismissible: true` so the
/// framework's dismiss action (Esc) is enabled.
class CreateSheetDialog extends StatelessWidget {
  const CreateSheetDialog({super.key, required this.templateId});

  /// Template the dialog was opened for (CAT-15).
  final String templateId;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    return AlertDialog(
      title: Text(localizations.createSheetTitle),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            localizations.createSheetTemplateLabel,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: AppSpacing.xs),
          Text(
            templateId,
            key: const Key('create-sheet-template-id'),
            style: theme.textTheme.bodyMedium,
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(localizations.actionClose),
        ),
      ],
    );
  }
}
