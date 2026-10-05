/// In-memory edit state for one open sheet (LGN-102, GRN-01 … GRN-06; LGN-110;
/// LGN-104: preview, rate limits, draft restore).
///
/// The family is keyed by the opened sheet id and autoDispose: the state lives
/// only while a sheet surface subscribes, so leaving the screen discards every
/// edit and a reopen reseeds from the stored sheet overlaid with any draft
/// backup. [SheetEditController.save] commits the changed editable delta and
/// merges the derived cascade the API returns, without reloading the sheet;
/// [SheetEditController.preview] runs the stateless dry-run and merges the
/// cascade without advancing the baseline.
///
/// Rebuild granularity comes from `select`: field controls watch a single
/// dotted path of this state, so one edit rebuilds exactly one control and
/// the sheet screen itself never watches the state (AD-001).
library;

import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_defaults.dart';
import 'package:legnaris/sheet/action_rate_limiter.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_open_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sheet_edit_provider.g.dart';

/// The stored sheet an open Screen 2 was launched for, injected per route.
///
/// The creation flow overrides this with the `201` payload so Screen 2 edits
/// the server-stored values without a second request (CRT-08, OPN-09); the
/// library path leaves it null and `SheetOpenController` fetches the sheet by
/// id. Overriding at the route scope keeps the screen, this controller and
/// every field consumer on one container (design: opened-sheet seam).
@Riverpod(keepAlive: true)
Sheet? openedSheet(Ref ref) => null;

/// Immutable values of one sheet's edit state, addressed by dotted path.
class SheetEditState {
  const SheetEditState({
    this.defaults = const {},
    this.data = const {},
    this.editedPaths = const {},
    this.isSaving = false,
    this.saveError,
    this.revision = 0,
    this.previewedRevision,
    this.isPreviewing = false,
    this.previewError,
    this.isPreviewBlocked = false,
    this.isSaveBlocked = false,
  });

  /// Reference values the state was seeded from: the template schema defaults,
  /// or the opened sheet's stored data when one moved into Screen 2. The
  /// baseline [isPending] compares against, advanced to the server state after
  /// a successful save.
  final Map<String, Object?> defaults;

  /// Current values. State maps are never mutated in place — every write
  /// produces a new tree (copy-on-write), so listeners can compare states.
  final Map<String, Object?> data;

  /// Paths written at least once since the seed. A path back at its default
  /// value no longer counts as pending.
  final Set<String> editedPaths;

  /// True while a `PATCH /sheets/:id` is in flight: the footer disables Save
  /// and the discard guards stand down so the commit is never abandoned.
  final bool isSaving;

  /// Mapped copy of the last save failure, rendered inline in the footer; null
  /// before any attempt and after a success.
  final String? saveError;

  /// Increments on every data-changing edit. The preview idempotency key: a
  /// repeat activation is blocked while it equals [previewedRevision].
  final int revision;

  /// Revision at the last successful preview; null before one and after a
  /// save. A failure never advances it, so Preview stays available.
  final int? previewedRevision;

  /// True while a `POST /sheets/:id/preview` is in flight.
  final bool isPreviewing;

  /// Mapped copy of the last preview failure, rendered inline in the footer;
  /// null before any attempt and after a success.
  final String? previewError;

  /// True while three preview requests were issued inside the rolling window.
  final bool isPreviewBlocked;

  /// True while three save requests were issued inside the rolling window.
  final bool isSaveBlocked;

  /// True while any edited path differs from its seeded reference: the marker,
  /// the Save action and both discard guards key off this (GRN-11 … GRN-19).
  bool get isPending => editedPaths.any(
    (path) => valueAtPath(data, path) != valueAtPath(defaults, path),
  );

  /// Nested delta of the changed leaves: every edited path whose value differs
  /// from the baseline, rebuilt by dotted path. Read-only controls never write,
  /// so a derived leaf is never in [editedPaths] and never enters the delta.
  Map<String, Object?> get pendingDelta {
    final delta = <String, Object?>{};
    for (final path in editedPaths) {
      final value = valueAtPath(data, path);
      if (value != valueAtPath(defaults, path)) {
        setPathValue(delta, path, value);
      }
    }
    return delta;
  }

  /// True while any request is in flight: Preview and Save block together so
  /// an older cascade can never merge over a newer baseline (SVE-01).
  bool get isBusy => isSaving || isPreviewing;

  /// Preview is offered only when there is a new edit to dry-run and neither
  /// action is in flight or rate-limited (PRV-01, PRV-02).
  bool get canPreview =>
      isPending &&
      previewedRevision != revision &&
      !isBusy &&
      !isPreviewBlocked;

  /// Save is offered only while edits are pending and no request is in flight
  /// or rate-limited (SVE-03).
  bool get canSave => isPending && !isBusy && !isSaveBlocked;

  /// Sentinel distinguishing "argument omitted" from "explicitly set to null"
  /// in [copyWith] for the nullable fields.
  static const Object _unset = Object();

  /// Returns a copy with the given fields replaced, so every emission stays
  /// consistent and omitting a field preserves its current value.
  SheetEditState copyWith({
    Map<String, Object?>? defaults,
    Map<String, Object?>? data,
    Set<String>? editedPaths,
    bool? isSaving,
    Object? saveError = _unset,
    int? revision,
    Object? previewedRevision = _unset,
    bool? isPreviewing,
    Object? previewError = _unset,
    bool? isPreviewBlocked,
    bool? isSaveBlocked,
  }) => SheetEditState(
    defaults: defaults ?? this.defaults,
    data: data ?? this.data,
    editedPaths: editedPaths ?? this.editedPaths,
    isSaving: isSaving ?? this.isSaving,
    saveError: saveError == _unset ? this.saveError : saveError as String?,
    revision: revision ?? this.revision,
    previewedRevision: previewedRevision == _unset
        ? this.previewedRevision
        : previewedRevision as int?,
    isPreviewing: isPreviewing ?? this.isPreviewing,
    previewError: previewError == _unset
        ? this.previewError
        : previewError as String?,
    isPreviewBlocked: isPreviewBlocked ?? this.isPreviewBlocked,
    isSaveBlocked: isSaveBlocked ?? this.isSaveBlocked,
  );
}

/// Edit state of the sheet opened for one sheet id.
///
/// Rebuilds (and reseeds) whenever the open sheet or the template detail
/// changes: the loading state drops the value, which discards the edits, and a
/// successful load seeds from the sheet's stored data over the schema defaults
/// — the discard-confirmed refresh flow rides on that (GRN-17). A draft backup
/// is overlaid on that seed, filtered to the template's editable leaves
/// (DRB-06, DRB-07).
@riverpod
class SheetEditController extends _$SheetEditController {
  /// Spec rate limit: three requests per rolling 60-second window (SVE-02).
  static const int _maxRequestsPerWindow = 3;
  static const Duration _rateWindow = Duration(seconds: 60);

  late final RollingWindowLimiter _previewLimiter = RollingWindowLimiter(
    maxRequests: _maxRequestsPerWindow,
    window: _rateWindow,
    now: ref.read(clockProvider),
  );

  late final RollingWindowLimiter _saveLimiter = RollingWindowLimiter(
    maxRequests: _maxRequestsPerWindow,
    window: _rateWindow,
    now: ref.read(clockProvider),
  );

  @override
  SheetEditState build(String sheetId) {
    final sheet = ref.watch(sheetOpenControllerProvider(sheetId)).sheet;
    if (sheet == null) return const SheetEditState();
    final template = ref
        .watch(templateDetailControllerProvider(sheet.templateId))
        .template;
    if (template == null) return const SheetEditState();
    final seed = seedFromSheet(template.fields, sheet.data);
    final restored = _restoreDraft(template.fields, seed);
    return SheetEditState(
      defaults: seed,
      data: restored.data,
      editedPaths: restored.paths,
      isPreviewBlocked: _previewLimiter.isBlocked,
      isSaveBlocked: _saveLimiter.isBlocked,
    );
  }

  /// Stores [value] at the dotted [path], keeping the previous state's maps
  /// untouched and recording the path for [SheetEditState.isPending]. The
  /// revision advances only when the value actually changes, so a redundant
  /// write never invalidates a successful preview.
  void setPath(String path, Object? value) {
    final changed = valueAtPath(state.data, path) != value;
    final data = _copyTree(state.data);
    setPathValue(data, path, value);
    state = state.copyWith(
      data: data,
      editedPaths: {...state.editedPaths, path},
      revision: changed ? state.revision + 1 : state.revision,
    );
  }

  /// Stores [selectedPath] as the active option of an exclusive group and every
  /// [siblings] path as zero in one copy-on-write emission, so the group can
  /// never end with two members on (EXC-13). A null [selectedPath] clears the
  /// group: every [siblings] path is set to zero (the `None` selection). When no
  /// affected path changes (the option is already active, every sibling already
  /// zero) it returns without emitting, keeping the state instance identical
  /// (EXC-14). Every written path is recorded in [SheetEditState.editedPaths].
  void setExclusive(String? selectedPath, Iterable<String> siblings) {
    final paths = selectedPath == null
        ? siblings.toList()
        : <String>[selectedPath, ...siblings];
    final changed = paths.any(
      (path) =>
          valueAtPath(state.data, path) != (path == selectedPath ? 1 : 0),
    );
    if (!changed) return;
    final data = _copyTree(state.data);
    for (final path in paths) {
      setPathValue(data, path, path == selectedPath ? 1 : 0);
    }
    state = state.copyWith(
      data: data,
      editedPaths: {...state.editedPaths, ...paths},
      revision: state.revision + 1,
    );
  }

  /// Runs one stateless dry-run and merges the returned cascade (PRV-03 …
  /// PRV-09).
  ///
  /// No-op while a request is in flight, the action is rate-limited, there is
  /// nothing new to preview, or the delta is empty. On success the cascade is
  /// merged into [SheetEditState.data] only: the baseline never advances and
  /// every edit stays pending (PRV-05, PRV-06); the current revision is
  /// recorded so a repeat with no new edit is blocked (PRV-07). On failure the
  /// edits stay, the mapped copy is published, and the revision is not
  /// recorded, so Preview remains available (PRV-08, PRV-09).
  Future<void> preview() async {
    if (!state.canPreview) return;
    final delta = state.pendingDelta;
    if (delta.isEmpty) return;
    final attemptedRevision = state.revision;
    _previewLimiter.record();
    state = state.copyWith(
      isPreviewing: true,
      previewError: null,
      isPreviewBlocked: _previewLimiter.isBlocked,
      isSaveBlocked: _saveLimiter.isBlocked,
    );
    try {
      final cascade = await ref
          .read(apiClientProvider)
          .previewSheet(sheetId, delta);
      if (!ref.mounted) return;
      state = state.copyWith(
        data: mergeCascade(state.data, cascade),
        isPreviewing: false,
        previewError: null,
        previewedRevision: attemptedRevision,
        isPreviewBlocked: _previewLimiter.isBlocked,
        isSaveBlocked: _saveLimiter.isBlocked,
      );
    } catch (error) {
      if (!ref.mounted) return;
      state = state.copyWith(
        isPreviewing: false,
        previewError: failureCopy(error),
        isPreviewBlocked: _previewLimiter.isBlocked,
        isSaveBlocked: _saveLimiter.isBlocked,
      );
    }
    _armCooldown();
  }

  /// Commits the pending delta with one `PATCH /sheets/:id` and merges the
  /// derived cascade the API returns (SAV-08 … SAV-23).
  ///
  /// No-op while a request is in flight, the delta is empty, or the save
  /// window is exhausted, so Save and `Ctrl+S` can never double up and a
  /// clean sheet issues no request. The attempt is recorded in the save
  /// window (SVE-06). On success the baseline advances to what the server now
  /// stores, the recorded preview delta clears (PRV-10) and the draft backup
  /// is deleted (DRB-08). On failure every edit stays and the mapped copy is
  /// published; the PATCH is never retried automatically.
  Future<void> save() async {
    if (!state.canSave) return;
    final delta = state.pendingDelta;
    if (delta.isEmpty) return;
    _saveLimiter.record();
    state = state.copyWith(
      isSaving: true,
      isPreviewBlocked: _previewLimiter.isBlocked,
      isSaveBlocked: _saveLimiter.isBlocked,
    );
    try {
      final cascade = await ref
          .read(apiClientProvider)
          .patchSheet(sheetId, delta);
      if (!ref.mounted) return;
      _applySaved(delta, cascade);
    } catch (error) {
      if (!ref.mounted) return;
      state = state.copyWith(
        isSaving: false,
        saveError: failureCopy(error),
        isPreviewBlocked: _previewLimiter.isBlocked,
        isSaveBlocked: _saveLimiter.isBlocked,
      );
    }
    _armCooldown();
  }

  /// Merges the cascade into the working data and advances the baseline to the
  /// server state: the sent [delta] over the old baseline, then the [cascade].
  /// Clears the preview record and deletes the draft backup.
  void _applySaved(
    Map<String, Object?> delta,
    Map<String, Object?> cascade,
  ) {
    final data = mergeCascade(state.data, cascade);
    final baseline = mergeCascade(
      mergeCascade(state.defaults, delta),
      cascade,
    );
    state = state.copyWith(
      defaults: baseline,
      data: data,
      isSaving: false,
      saveError: null,
      previewedRevision: null,
      isPreviewBlocked: _previewLimiter.isBlocked,
      isSaveBlocked: _saveLimiter.isBlocked,
    );
    ref.read(draftBackupStoreProvider).delete(sheetId);
  }

  /// Reads the stored backup and overlays it on [seed], keeping only keys that
  /// are editable leaves of the loaded template. Returns the overlaid data and
  /// the restored paths to mark pending (DRB-06, DRB-07).
  ({Map<String, Object?> data, Set<String> paths}) _restoreDraft(
    FieldGroup fields,
    Map<String, Object?> seed,
  ) {
    final draft = ref.read(draftBackupStoreProvider).read(sheetId);
    if (draft == null || draft.isEmpty) {
      return (data: seed, paths: const <String>{});
    }
    final editablePaths = <String>{
      for (final section in buildSections(fields))
        for (final leaf in section.leaves)
          if (leaf.schema.isEditable) leaf.path,
    };
    final data = _copyTree(seed);
    final paths = <String>{};
    for (final entry in _flattenDraft(draft).entries) {
      if (!editablePaths.contains(entry.key)) continue;
      setPathValue(data, entry.key, entry.value);
      paths.add(entry.key);
    }
    return (data: data, paths: paths);
  }

  /// Schedules a one-shot re-check at the earliest instant a blocked action's
  /// window frees, so the action re-enables without waiting for a rebuild
  /// (SVE-04).
  void _armCooldown() {
    final previewNextFree = _previewLimiter.nextFreeAt();
    final saveNextFree = _saveLimiter.nextFreeAt();
    final candidates = <DateTime>[?previewNextFree, ?saveNextFree];
    if (candidates.isEmpty) {
      ref.read(cooldownSchedulerProvider).cancel();
      return;
    }
    candidates.sort();
    final delay = candidates.first.difference(ref.read(clockProvider)());
    ref
        .read(cooldownSchedulerProvider)
        .schedule(
          delay.isNegative ? Duration.zero : delay,
          _onCooldownElapsed,
        );
  }

  /// Recomputes the rate-limit flags when a window advances and re-arms for any
  /// action still blocked.
  void _onCooldownElapsed() {
    if (!ref.mounted) return;
    final previewBlocked = _previewLimiter.isBlocked;
    final saveBlocked = _saveLimiter.isBlocked;
    if (previewBlocked != state.isPreviewBlocked ||
        saveBlocked != state.isSaveBlocked) {
      state = state.copyWith(
        isPreviewBlocked: previewBlocked,
        isSaveBlocked: saveBlocked,
      );
    }
    _armCooldown();
  }
}

/// Flattens a nested value tree to leaf dotted paths, sharing scalar leaves.
Map<String, Object?> _flattenDraft(
  Map<String, Object?> source, [
  String prefix = '',
]) {
  final result = <String, Object?>{};
  for (final entry in source.entries) {
    final path = prefix.isEmpty ? entry.key : '$prefix.${entry.key}';
    final value = entry.value;
    if (value is Map<String, Object?> && value.isNotEmpty) {
      result.addAll(_flattenDraft(value, path));
    } else {
      result[path] = value;
    }
  }
  return result;
}

/// Deep copy of the nested value tree, sharing scalar leaves.
Map<String, Object?> _copyTree(Map<String, Object?> source) => {
  for (final entry in source.entries)
    entry.key: entry.value is Map<String, Object?>
        ? _copyTree(entry.value as Map<String, Object?>)
        : entry.value,
};
