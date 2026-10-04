/// Screen 2 (Tela 2) — the navigable sheet surface (RND-14 … RND-20, OPN-05 …).
///
/// Host for the UI Engine: it loads one stored sheet through
/// [sheetOpenControllerProvider], waits for the template detail its
/// `template_id` names, and renders [SheetForm] with an in-memory edit state
/// per field (LGN-102). Each control subscribes to its own dotted path through
/// a `select`, so an edit rebuilds exactly that control and nothing else;
/// values live in [sheetEditControllerProvider] keyed by the sheet id
/// (discarded when the screen unmounts) and no mutation is issued — the save
/// cycle is Phase 4. A header caption signals pending local edits and the back
/// action confirms before discarding them (GRN-11 … GRN-16); F5 refreshes the
/// template behind the same kind of confirmation (GRN-17 … GRN-19).
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/draft_auto_backup.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_open_provider.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 2 for one stored sheet.
class SheetScreen extends ConsumerStatefulWidget {
  const SheetScreen({super.key, required this.sheetId});

  /// Stored sheet the screen renders; its `template_id` selects the fields.
  final String sheetId;

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
        ref.read(activeSheetIdProvider.notifier).set(widget.sheetId);
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
    // Ctrl+S dispatch (SAV-05): the shell bumps the save tick, this screen
    // runs the commit. The edit controller owns the single-flight guard.
    ref.listen(sheetSaveTickProvider, (previous, next) {
      if (previous == null || next > previous) _save();
    });
    // Holds the active-sheet registration from the first build until unmount,
    // where the element closes it and the provider discards the id.
    ref.listen(activeSheetIdProvider, (_, _) {});
    final localizations = AppLocalizations.of(context)!;
    final open = ref.watch(sheetOpenControllerProvider(widget.sheetId));
    final templateId = open.sheet?.templateId;
    final detail = templateId == null
        ? null
        : ref.watch(templateDetailControllerProvider(templateId));

    final template = detail?.template;
    final loaded =
        open.sheet != null &&
        open.errorMessage == null &&
        !open.isLoading &&
        detail != null &&
        !detail.isLoading &&
        template != null &&
        buildSections(template.fields).isNotEmpty;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _header(
          context,
          open.sheet?.name ?? '',
          templateId ?? '',
          localizations,
        ),
        const Divider(),
        Expanded(child: _body(context, open, detail, localizations)),
        if (loaded) ...[
          const Divider(),
          _footer(context, localizations),
        ],
      ],
    );
  }

  Widget _header(
    BuildContext context,
    String title,
    String templateId,
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
            title,
            key: const Key('sheet-title'),
            style: theme.textTheme.titleLarge,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        Text(templateId, style: muted),
      ],
    );
  }

  /// Commits the pending delta through the edit controller (SAV-04). The
  /// controller owns the single-flight guard, so a double activation or a
  /// `Ctrl+S` in the same frame cannot issue two PATCHes.
  void _save() {
    unawaited(
      ref.read(sheetEditControllerProvider(widget.sheetId).notifier).save(),
    );
  }

  /// Runs one stateless dry-run through the edit controller (PRV-03). The
  /// controller owns the single-flight and rate-limit guards, so a repeat with
  /// no new edit issues no request.
  void _preview() {
    unawaited(
      ref.read(sheetEditControllerProvider(widget.sheetId).notifier).preview(),
    );
  }

  /// The fixed footer: the dirty indicator, the mapped save and preview errors
  /// above the action row, and the Preview + Save actions (PRV-01 … PRV-04,
  /// SVE-01 … SVE-03, SAV-01 … SAV-06). Its own `Consumer` selecting only the
  /// action flags keeps the form out of the rebuild when a flag flips (AD-001)
  /// and keeps the periodic draft backup alive while the loaded sheet is on
  /// screen (DRB-01, DRB-02).
  Widget _footer(BuildContext context, AppLocalizations localizations) {
    final theme = Theme.of(context);
    return Consumer(
      builder: (context, ref, _) {
        final provider = sheetEditControllerProvider(widget.sheetId);
        ref.watch(draftAutoBackupProvider(widget.sheetId));
        final pending = ref.watch(provider.select((state) => state.isPending));
        final busy = ref.watch(provider.select((state) => state.isBusy));
        final canPreview = ref.watch(
          provider.select((state) => state.canPreview),
        );
        final canSave = ref.watch(provider.select((state) => state.canSave));
        final previewBlocked = ref.watch(
          provider.select((state) => state.isPreviewBlocked),
        );
        final saveBlocked = ref.watch(
          provider.select((state) => state.isSaveBlocked),
        );
        final previewError = ref.watch(
          provider.select((state) => state.previewError),
        );
        final saveError = ref.watch(provider.select((state) => state.saveError));
        final previewButton = TextButton(
          key: const Key('sheet-preview'),
          onPressed: canPreview ? _preview : null,
          child: Text(localizations.sheetPreviewAction),
        );
        final saveButton = TextButton(
          key: const Key('sheet-save'),
          onPressed: canSave ? _save : null,
          child: Text(localizations.sheetSaveAction),
        );
        return Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.md,
            vertical: AppSpacing.sm,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (previewError != null) ...[
                Text(
                  previewError,
                  key: const Key('sheet-preview-error'),
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
              ],
              if (saveError != null) ...[
                Text(
                  saveError,
                  key: const Key('sheet-save-error'),
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
              ],
              Row(
                children: [
                  if (pending)
                    Text(
                      localizations.sheetLocalEdits,
                      key: const Key('sheet-local-edits'),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.primary,
                      ),
                    ),
                  const Spacer(),
                  if (!canPreview)
                    Tooltip(
                      message: _previewDisabledTooltip(
                        localizations,
                        busy: busy,
                        blocked: previewBlocked,
                      ),
                      child: previewButton,
                    )
                  else
                    previewButton,
                  const SizedBox(width: AppSpacing.sm),
                  if (!canSave)
                    Tooltip(
                      message: _saveDisabledTooltip(
                        localizations,
                        busy: busy,
                        blocked: saveBlocked,
                      ),
                      child: saveButton,
                    )
                  else
                    saveButton,
                ],
              ),
            ],
          ),
        );
      },
    );
  }

  /// The reason Preview is disabled, in priority order: a request is in
  /// flight, the preview window is exhausted, or nothing changed since the
  /// last preview (PRV-02, SVE-01, SVE-02).
  String _previewDisabledTooltip(
    AppLocalizations localizations, {
    required bool busy,
    required bool blocked,
  }) {
    if (busy) return localizations.sheetRequestInProgressTooltip;
    if (blocked) return localizations.sheetPreviewLimitTooltip;
    return localizations.sheetPreviewNoChangesTooltip;
  }

  /// The reason Save is disabled, in priority order: a request is in flight,
  /// the save window is exhausted, or there are no changes to save (SVE-01,
  /// SVE-03).
  String _saveDisabledTooltip(
    AppLocalizations localizations, {
    required bool busy,
    required bool blocked,
  }) {
    if (busy) return localizations.sheetRequestInProgressTooltip;
    if (blocked) return localizations.sheetSaveLimitTooltip;
    return localizations.sheetSaveNoChangesTooltip;
  }

  /// Back action: a clean sheet pops directly (GRN-16); a pending one asks
  /// first, because the edit state lives only in memory and popping would
  /// silently discard it (GRN-13 … GRN-15). The dirty check is a read at tap
  /// time — the screen never watches the edit state (AD-001).
  Future<void> _leave() async {
    if (_confirming) return;
    final editProvider = sheetEditControllerProvider(widget.sheetId);
    // An in-flight preview or save owns the state until its response settles;
    // leaving now would hide the result and desync the screen (SVE-01, SAV-06).
    if (ref.read(editProvider).isBusy) return;
    final pending = ref.read(editProvider).isPending;
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
    final editProvider = sheetEditControllerProvider(widget.sheetId);
    // F5 must not discard edits while a preview or save is in flight (SVE-01,
    // SAV-06): the request owns the state until its response settles.
    if (ref.read(editProvider).isBusy) return;
    final templateId = ref
        .read(sheetOpenControllerProvider(widget.sheetId))
        .sheet
        ?.templateId;
    if (templateId == null) return;
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
        .read(templateDetailControllerProvider(templateId).notifier)
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

  /// Picks the render state: the sheet load, then the template detail, then
  /// the form. `NOT_FOUND` (already evicted) renders the missing-sheet copy
  /// with no Retry (OPN-06); every other failure offers Retry (OPN-07, OPN-08).
  Widget _body(
    BuildContext context,
    SheetOpenState open,
    TemplateDetailState? detail,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    final muted = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    if (open.isLoading) {
      return _loading(context, localizations.sheetLoading, muted);
    }

    if (open.errorMessage != null) {
      return _error(
        context,
        open.errorMessage!,
        onRetry: open.isNotFound
            ? null
            : () => ref
                  .read(sheetOpenControllerProvider(widget.sheetId).notifier)
                  .load(),
        localizations: localizations,
      );
    }

    final sheet = open.sheet!;
    final detailState = detail!;
    if (detailState.isLoading) {
      return _loading(context, localizations.templateDetailLoading, muted);
    }

    final template = detailState.template;
    if (template == null) {
      final message = detailState.errorMessage;
      if (message == null) return const SizedBox.shrink();
      return _error(
        context,
        message,
        onRetry: () => ref
            .read(templateDetailControllerProvider(sheet.templateId).notifier)
            .load(),
        localizations: localizations,
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

  Widget _loading(
    BuildContext context,
    String message,
    TextStyle? muted,
  ) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      const LinearProgressIndicator(),
      const SizedBox(height: AppSpacing.sm),
      Text(message, style: muted),
    ],
  );

  /// A failure row: the mapped copy plus Retry when the failure is retryable.
  Widget _error(
    BuildContext context,
    String message, {
    required VoidCallback? onRetry,
    required AppLocalizations localizations,
  }) {
    final theme = Theme.of(context);
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
        if (onRetry != null) ...[
          const SizedBox(width: AppSpacing.md),
          OutlinedButton(
            onPressed: onRetry,
            child: Text(localizations.actionRetry),
          ),
        ],
      ],
    );
  }

  /// One field's control, wired to the edit state: the `Consumer` subscribes
  /// to this field's dotted path only, so an edit to any other field leaves
  /// this subtree untouched (GRN-07, GRN-08). The screen itself never
  /// watches the edit state (AD-001).
  Widget _control(BuildContext context, SheetField field) => Consumer(
    builder: (context, ref, _) {
      final provider = sheetEditControllerProvider(widget.sheetId);
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
  /// Selecting writes the exclusivity rule into the state — the selected member
  /// to 1 and every sibling to 0, or every member to 0 for the `None` sentinel
  /// — with no network request (EXC-13, EXC-15, EXC-16, EXC-27).
  Widget _exclusiveControl(
    BuildContext context,
    SheetExclusiveGroup group,
    String? selectedPath,
    ValueChanged<String>? onSelect,
  ) => Consumer(
    key: ValueKey('sheet-exclusive-consumer-${group.id}'),
    builder: (context, ref, _) {
      final provider = sheetEditControllerProvider(widget.sheetId);
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
          path == exclusiveNoneValue ? null : path,
          group.options
              .map((option) => option.path)
              .where((optionPath) => optionPath != path),
        ),
      );
    },
  );
}
