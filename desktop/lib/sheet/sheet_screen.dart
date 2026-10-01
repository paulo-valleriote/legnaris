/// Screen 2 (Tela 2) — the navigable sheet surface (RND-14 … RND-20).
///
/// Host for the UI Engine: it fetches the template detail on demand and
/// renders [SheetForm] with an in-memory edit state per field (LGN-102).
/// Each control subscribes to its own dotted path through a `select`, so an
/// edit rebuilds exactly that control and nothing else; values live in
/// [sheetEditControllerProvider] (discarded when the screen unmounts) and no
/// mutation is issued — the save cycle is Phase 4.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 2 for one template.
class SheetScreen extends ConsumerStatefulWidget {
  const SheetScreen({super.key, required this.templateId});

  /// Template whose fields are rendered (CAT-15).
  final String templateId;

  @override
  ConsumerState<SheetScreen> createState() => _SheetScreenState();
}

class _SheetScreenState extends ConsumerState<SheetScreen> {
  @override
  Widget build(BuildContext context) {
    final localizations = AppLocalizations.of(context)!;
    final state = ref.watch(
      templateDetailControllerProvider(widget.templateId),
    );
    final template = state.template;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _header(context, template),
        const Divider(),
        Expanded(child: _body(context, state, localizations)),
      ],
    );
  }

  Widget _header(BuildContext context, TemplateDetail? template) {
    final theme = Theme.of(context);
    final muted = theme.textTheme.bodySmall?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    return Row(
      children: [
        IconButton(
          key: const Key('sheet-back'),
          icon: const Icon(Icons.arrow_back),
          tooltip: MaterialLocalizations.of(context).backButtonTooltip,
          onPressed: () => Navigator.of(context).maybePop(),
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: Text(
            template?.name ?? '',
            key: const Key('sheet-title'),
            style: theme.textTheme.titleLarge,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        Text(widget.templateId, style: muted),
      ],
    );
  }

  Widget _body(
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
        crossAxisAlignment: CrossAxisAlignment.stretch,
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

    if (buildSections(template.fields).isEmpty) {
      return Align(
        alignment: Alignment.topLeft,
        child: Text(localizations.templateNoFields, style: muted),
      );
    }

    return SingleChildScrollView(
      child: SheetForm(fields: template.fields, controlBuilder: _control),
    );
  }

  /// One field's control, wired to the edit state: the `Consumer` subscribes
  /// to this field's dotted path only, so an edit to any other field leaves
  /// this subtree untouched (GRN-07, GRN-08). The screen itself never
  /// watches the edit state (AD-001).
  Widget _control(BuildContext context, SheetField field) => Consumer(
    builder: (context, ref, _) {
      final provider = sheetEditControllerProvider(widget.templateId);
      final value = ref.watch(
        provider.select((state) => valueAtPath(state.data, field.path)),
      );
      return SheetFieldControl(
        path: field.path,
        schema: field.schema,
        value: value,
        onChanged: (value) =>
            ref.read(provider.notifier).setPath(field.path, value),
      );
    },
  );
}
