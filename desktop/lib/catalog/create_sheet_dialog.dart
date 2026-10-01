import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_defaults.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 3 — the creation surface (TPL-01 … TPL-14).
///
/// Two columns: the dialog information (template identity + sheet-name input)
/// on the left, and a read-only preview of the template fields seeded from
/// their schema defaults on the right (LGN-100, PRV-01 … PRV-13). Opening it
/// issues no mutation (TPL-13); the preview never writes either.
class CreateSheetDialog extends ConsumerStatefulWidget {
  const CreateSheetDialog({super.key, required this.templateId});

  /// Template the dialog was opened for (CAT-15).
  final String templateId;

  @override
  ConsumerState<CreateSheetDialog> createState() => _CreateSheetDialogState();
}

class _CreateSheetDialogState extends ConsumerState<CreateSheetDialog> {
  final _nameController = TextEditingController();
  final _previewScrollController = ScrollController();

  @override
  void dispose() {
    _nameController.dispose();
    _previewScrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final localizations = AppLocalizations.of(context)!;
    final state = ref.watch(templateDetailControllerProvider(widget.templateId));

    return AlertDialog(
      content: SizedBox(
        width: AppSizes.createSheetDialogWidth,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _information(context, state, localizations),
            const SizedBox(height: AppSpacing.lg),
            _preview(context, state, localizations),
          ],
        ),
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

  /// The identity block: template label, the `name | id` row and the sheet-name
  /// input. The preview card renders below it.
  Widget _information(
    BuildContext context,
    TemplateDetailState state,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    final muted = theme.textTheme.bodySmall?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(localizations.createSheetTemplateLabel, style: muted),
        const SizedBox(height: AppSpacing.xs),
        _identity(context, state.template?.name),
        const SizedBox(height: AppSpacing.lg),
        Text(localizations.createSheetNameLabel, style: muted),
        const SizedBox(height: AppSpacing.xs),
        TextField(
          key: const Key('create-sheet-name-input'),
          controller: _nameController,
          decoration: InputDecoration(
            hintText: localizations.createSheetNamePlaceholder,
          ),
        ),
      ],
    );
  }

  /// Template name and id on one line, separated by a hairline divider; the
  /// divider is omitted when the name is empty, so the row never shows a
  /// dangling separator.
  Widget _identity(BuildContext context, String? name) {
    final theme = Theme.of(context);
    final hasName = name != null && name.isNotEmpty;
    return Row(
      children: [
        if (hasName) ...[
          Flexible(
            child: Text(
              name,
              style: theme.textTheme.titleMedium,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          const SizedBox(width: AppSpacing.sm),
          Container(
            key: const Key('create-sheet-template-id-divider'),
            width: AppSizes.hairline,
            height: AppSpacing.lg,
            color: theme.colorScheme.outlineVariant,
          ),
          const SizedBox(width: AppSpacing.sm),
        ],
        Text(
          widget.templateId,
          key: const Key('create-sheet-template-id'),
          style: theme.textTheme.bodyMedium,
        ),
      ],
    );
  }

  /// The right column: the always-visible, read-only preview card. The loading,
  /// mapped-error and empty states render inside the same card (PRV-04, PRV-07,
  /// PRV-09).
  Widget _preview(
    BuildContext context,
    TemplateDetailState state,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return Card(
      key: const Key('create-sheet-preview'),
      margin: EdgeInsets.zero,
      elevation: 0,
      shape: RoundedRectangleBorder(
        side: BorderSide(
          color: theme.colorScheme.outlineVariant,
          width: AppSizes.hairline,
        ),
        borderRadius: BorderRadius.circular(AppRadius.panel),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              localizations.createSheetPreviewLabel,
              key: const Key('create-sheet-preview-label'),
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
            ConstrainedBox(
              constraints: const BoxConstraints(
                maxHeight: AppSizes.previewCardMaxHeight,
              ),
              child: Scrollbar(
                controller: _previewScrollController,
                thumbVisibility: true,
                child: SingleChildScrollView(
                  controller: _previewScrollController,
                  padding: const EdgeInsets.only(right: AppSpacing.sm),
                  child: _previewBody(context, state, localizations),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _previewBody(
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
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            message,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.error,
            ),
          ),
          const SizedBox(height: AppSpacing.sm),
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

    if (buildSections(template.fields).isEmpty) {
      return Text(localizations.templateNoFields, style: muted);
    }

    return SheetForm(
      fields: template.fields,
      data: seedDefaults(template.fields),
      onChanged: null,
    );
  }
}
