/// Screen 2 (Tela 2) — the navigable sheet surface (RND-14 … RND-20).
///
/// Host for the UI Engine: it fetches the template detail on demand and
/// renders [SheetForm] with an in-memory edit state per field (LGN-102).
/// Each control subscribes to its own dotted path through a `select`, so an
/// edit rebuilds exactly that control and nothing else; values live in
/// [sheetEditControllerProvider] (discarded when the screen unmounts) and no
/// mutation is issued — the save cycle is Phase 4. A header caption signals
/// pending local edits and the back action confirms before discarding them
/// (GRN-11 … GRN-16); F5 refreshes the template behind the same kind of
/// confirmation (GRN-17 … GRN-19).
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';
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
  /// True while a discard confirmation is on screen: a modal barrier blocks
  /// the back button, but F5 dispatches through the provider, so the guards
  /// check this flag themselves to keep one confirmation at a time.
  bool _confirming = false;

  @override
  void initState() {
    super.initState();
    // Register for the shell's F5 dispatch. The write cannot happen inside a
    // widget lifecycle — Riverpod forbids modifying providers while the tree
    // builds — so it lands right after the first frame, by which time the
    // build-time keep-alive listen already holds the provider; unmounting
    // closes that listen and discards the registration (GRN-17).
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        ref.read(activeSheetIdProvider.notifier).set(widget.templateId);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    // F5 dispatch (GRN-17 … GRN-19): the shell bumps the tick, this screen
    // runs the refresh. Re-listened every build because the element closes
    // these subscriptions whenever it rebuilds.
    ref.listen(sheetRefreshTickProvider, (previous, next) {
      if (previous == null || next > previous) unawaited(_refreshTemplate());
    });
    // Holds the active-sheet registration from the first build until unmount,
    // where the element closes it and the provider discards the id.
    ref.listen(activeSheetIdProvider, (_, _) {});
    final localizations = AppLocalizations.of(context)!;
    final state = ref.watch(
      templateDetailControllerProvider(widget.templateId),
    );
    final template = state.template;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _header(context, template, localizations),
        const Divider(),
        Expanded(child: _body(context, state, localizations)),
      ],
    );
  }

  Widget _header(
    BuildContext context,
    TemplateDetail? template,
    AppLocalizations localizations,
  ) {
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
          onPressed: _leave,
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
        _localEditsMarker(context, localizations),
        Text(widget.templateId, style: muted),
      ],
    );
  }

  /// Caption shown while the edit state differs from the seeded defaults
  /// (GRN-11, GRN-12). Its own `Consumer` so the screen build never
  /// subscribes to the edit state (AD-001): only this small subtree rebuilds
  /// when the pending flag flips.
  Widget _localEditsMarker(
    BuildContext context,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return Consumer(
      builder: (context, ref, _) {
        final pending = ref.watch(
          sheetEditControllerProvider(widget.templateId)
              .select((state) => state.isPending),
        );
        if (!pending) return const SizedBox.shrink();
        return Padding(
          padding: const EdgeInsets.only(right: AppSpacing.sm),
          child: Text(
            localizations.sheetLocalEdits,
            key: const Key('sheet-local-edits'),
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.primary,
            ),
          ),
        );
      },
    );
  }

  /// Back action: a clean sheet pops directly (GRN-16); a pending one asks
  /// first, because the edit state lives only in memory and popping would
  /// silently discard it (GRN-13 … GRN-15). The dirty check is a read at tap
  /// time — the screen never watches the edit state (AD-001).
  Future<void> _leave() async {
    if (_confirming) return;
    final pending = ref
        .read(sheetEditControllerProvider(widget.templateId))
        .isPending;
    if (pending) {
      final localizations = AppLocalizations.of(context)!;
      final confirmed = await _confirmDiscard(
        title: localizations.sheetLeaveDialogTitle,
        message: localizations.sheetLeaveDialogMessage,
        confirmLabel: localizations.sheetLeaveConfirmAction,
      );
      if (!mounted || !confirmed) return;
    }
    await Navigator.of(context).maybePop();
  }

  /// F5 refresh (GRN-17 … GRN-19): a clean sheet refetches directly; a
  /// pending one confirms first, and only a confirm discards the in-memory
  /// edits before the reload — cancel issues no request at all. The reload
  /// reseeds the edit state from the fresh schema; a failure leaves it
  /// discarded behind the error + Retry branch.
  Future<void> _refreshTemplate() async {
    if (_confirming) return;
    final editProvider = sheetEditControllerProvider(widget.templateId);
    if (ref.read(editProvider).isPending) {
      final localizations = AppLocalizations.of(context)!;
      final confirmed = await _confirmDiscard(
        title: localizations.sheetRefreshDialogTitle,
        message: localizations.sheetRefreshDialogMessage,
        confirmLabel: localizations.sheetRefreshConfirmAction,
      );
      if (!mounted || !confirmed) return;
      ref.invalidate(editProvider);
    }
    if (!mounted) return;
    await ref
        .read(templateDetailControllerProvider(widget.templateId).notifier)
        .load();
  }

  /// The destructive confirmation shared by both discard guards: states the
  /// exact consequence, and only an explicit confirm proceeds — barrier, Esc,
  /// or Cancel land on the safe side (desktop/AGENTS.md: never confirm by
  /// Enter alone). [_confirming] keeps a second guard from stacking while
  /// this dialog is up.
  Future<bool> _confirmDiscard({
    required String title,
    required String message,
    required String confirmLabel,
  }) async {
    _confirming = true;
    try {
      final confirmed = await showDialog<bool>(
        context: context,
        // Load-bearing: the framework gates Esc on this flag, and a barrier
        // or Esc tap lands on the safe side — dismiss equals cancel.
        barrierDismissible: true,
        builder: (dialogContext) {
          final localizations = AppLocalizations.of(dialogContext)!;
          final theme = Theme.of(dialogContext);
          return AlertDialog(
            title: Text(title),
            content: Text(message),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                child: Text(localizations.sheetLeaveCancelAction),
              ),
              TextButton(
                style: TextButton.styleFrom(
                  foregroundColor: theme.colorScheme.error,
                ),
                onPressed: () => Navigator.of(dialogContext).pop(true),
                child: Text(confirmLabel),
              ),
            ],
          );
        },
      );
      return confirmed == true;
    } finally {
      _confirming = false;
    }
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
      child: SheetForm(
        fields: template.fields,
        controlBuilder: _control,
        exclusiveControlBuilder: _exclusiveControl,
      ),
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

  /// One exclusive group's control, wired to the edit state: the `Consumer`
  /// watches every option path through a `select`, so an edit to a member
  /// rebuilds just this group and the screen never watches the state (AD-001).
  /// Selecting writes the exclusivity rule into the state — the selected
  /// member to 1 and every sibling to 0 — with no network request
  /// (EXC-13, EXC-15, EXC-16).
  Widget _exclusiveControl(
    BuildContext context,
    SheetExclusiveGroup group,
    String? selectedPath,
    ValueChanged<String>? onSelect,
  ) => Consumer(
    key: ValueKey('sheet-exclusive-consumer-${group.id}'),
    builder: (context, ref, _) {
      final provider = sheetEditControllerProvider(widget.templateId);
      String? active;
      for (final option in group.options) {
        final value = ref.watch(
          provider.select((state) => valueAtPath(state.data, option.path)),
        );
        if (exclusiveValueIsOn(value)) active = option.path;
      }
      return SheetExclusiveControl(
        group: group,
        selectedPath: active,
        onSelect: (path) => ref.read(provider.notifier).setExclusive(
          path,
          group.options.map((option) => option.path).where((p) => p != path),
        ),
      );
    },
  );
}
