import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_defaults.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 3 — the creation surface (TPL-01 … TPL-14).
///
/// One column: the dialog information (template identity + sheet-name input)
/// and, below it, a read-only preview of the template fields seeded from their
/// schema defaults (LGN-100, PRV-01 … PRV-13). Opening it issues no mutation
/// (TPL-13); the preview never writes either.
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

  /// True while a create request is in flight; disables the primary action so
  /// a double activation issues no second POST (CRT-02).
  bool _isSubmitting = false;

  /// Mapped copy of the last create failure, rendered inline (CRT-06).
  String? _errorMessage;

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
        key: const Key('create-sheet-content'),
        width: AppSizes.createSheetDialogWidth,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (_isSubmitting) ...[
              const LinearProgressIndicator(key: Key('create-sheet-progress')),
              const SizedBox(height: AppSpacing.sm),
            ],
            _information(context, state, localizations),
            const SizedBox(height: AppSpacing.lg),
            _preview(context, state, localizations),
            if (_errorMessage != null) ...[
              const SizedBox(height: AppSpacing.md),
              Text(
                _errorMessage!,
                key: const Key('create-sheet-error'),
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.error,
                ),
              ),
            ],
          ],
        ),
      ),
      actions: [
        _primaryAction(localizations),
        TextButton(
          onPressed: _isSubmitting ? null : () => Navigator.of(context).pop(),
          child: Text(localizations.actionClose),
        ),
      ],
    );
  }

  /// The dialog's primary action: inert until a non-empty sheet name is
  /// entered and while a create request is in flight, so a double activation
  /// can never issue a second POST (CRT-01, CRT-02, CRT-05). The
  /// `ValueListenableBuilder` is what makes the button react to typing
  /// without rebuilding the whole dialog.
  Widget _primaryAction(AppLocalizations localizations) {
    return ValueListenableBuilder<TextEditingValue>(
      valueListenable: _nameController,
      builder: (context, value, _) {
        final hasName = value.text.trim().isNotEmpty;
        final button = TextButton(
          key: const Key('create-sheet-submit'),
          onPressed: hasName && !_isSubmitting ? _submit : null,
          child: Text(localizations.createSheetPrimaryAction),
        );
        if (!hasName) {
          return Tooltip(
            message: localizations.createSheetNameRequiredTooltip,
            child: button,
          );
        }
        return button;
      },
    );
  }

  /// Creates the sheet: exactly one `POST /sheets` with the trimmed name, a
  /// best-effort local registration, then pops carrying the created sheet
  /// (CRT-01, CRT-03, CRT-04). A failure keeps the dialog open with the mapped
  /// copy so the next activation is a clean retry (CRT-06, CRT-07).
  Future<void> _submit() async {
    final name = _nameController.text.trim();
    if (_isSubmitting || name.isEmpty) return;
    setState(() {
      _isSubmitting = true;
      _errorMessage = null;
    });
    try {
      final sheet = await ref
          .read(apiClientProvider)
          .createSheet(templateId: widget.templateId, sheetName: name);
      _registerInLibrary(sheet);
      if (!mounted) return;
      Navigator.of(context).pop(sheet);
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _errorMessage = failureCopy(error);
      });
    }
  }

  /// Records the created sheet in the local index. A write failure is
  /// non-fatal: the `201` is authoritative and retrying would create a second
  /// sheet on the server (CRT-11).
  void _registerInLibrary(Sheet sheet) {
    final entry = SheetLibraryEntry(
      id: sheet.id,
      name: sheet.name,
      templateId: sheet.templateId,
      createdAt: sheet.createdAt,
    );
    try {
      ref.read(sheetLibraryProvider.notifier).upsert(entry);
    } on SheetLibraryException {
      return;
    }
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

  /// The always-visible, read-only preview card below the sheet-name input.
  /// The loading, mapped-error and empty states render inside the same card
  /// (PRV-04, PRV-07, PRV-09); its body scrolls with an always-visible bar
  /// (PRV-10).
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
