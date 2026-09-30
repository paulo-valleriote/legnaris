import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/catalog/template_summary.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 3 — the creation surface (TPL-01 … TPL-14).
///
/// Renders the on-demand template detail fetched by
/// [templateDetailControllerProvider]: template identity, prefix summary,
/// sheet-name input and a disabled primary action. Opening it issues no
/// mutation (TPL-13); the dialog content belongs to the template-detail spec.
class CreateSheetDialog extends ConsumerStatefulWidget {
  const CreateSheetDialog({super.key, required this.templateId});

  /// Template the dialog was opened for (CAT-15).
  final String templateId;

  @override
  ConsumerState<CreateSheetDialog> createState() => _CreateSheetDialogState();
}

class _CreateSheetDialogState extends ConsumerState<CreateSheetDialog> {
  final _nameController = TextEditingController();

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final state = ref.watch(templateDetailControllerProvider(widget.templateId));

    return AlertDialog(
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
          if (state.template != null) ...[
            Text(
              state.template!.name,
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: AppSpacing.xs),
          ],
          Text(
            widget.templateId,
            key: const Key('create-sheet-template-id'),
            style: theme.textTheme.bodyMedium,
          ),
          const SizedBox(height: AppSpacing.lg),
          _detail(context, state, localizations),
          const SizedBox(height: AppSpacing.lg),
          Text(
            localizations.createSheetNameLabel,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: AppSpacing.xs),
          TextField(
            controller: _nameController,
            decoration: InputDecoration(
              hintText: localizations.createSheetNamePlaceholder,
            ),
          ),
        ],
      ),
      actions: [
        Tooltip(
          message: localizations.createSheetCreateTooltip,
          child: TextButton(
            onPressed: null,
            child: Text(localizations.createSheetPrimaryAction),
          ),
        ),
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(localizations.actionClose),
        ),
      ],
    );
  }

  /// The detail area: a loading state, the mapped inline error with Retry, the
  /// prefix summary, or the empty-fields line (TPL-02, TPL-04, TPL-07 … TPL-10).
  Widget _detail(
    BuildContext context,
    TemplateDetailState state,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    final muted = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    if (state.isLoading) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const LinearProgressIndicator(),
          const SizedBox(height: AppSpacing.sm),
          Text(localizations.templateDetailLoading, style: muted),
        ],
      );
    }

    final template = state.template;
    if (template == null) {
      final message = state.errorMessage;
      if (message == null) return const SizedBox.shrink();
      return Row(
        children: [
          Expanded(
            child: Text(
              message,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          OutlinedButton(
            onPressed: () => ref
                .read(
                  templateDetailControllerProvider(widget.templateId).notifier,
                )
                .load(),
            child: Text(localizations.actionRetry),
          ),
        ],
      );
    }

    final summary = summarizeFields(template.fields);
    if (summary.isEmpty) {
      return Text(localizations.templateNoFields, style: muted);
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final line in summary)
          Text(
            '${line.prefix}: ${line.leafCount}',
            style: theme.textTheme.bodyMedium,
          ),
      ],
    );
  }
}
