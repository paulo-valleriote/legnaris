/// In-memory edit state for one open sheet (LGN-102, GRN-01 … GRN-06; LGN-110).
///
/// The family is keyed by the opened sheet id and autoDispose: the state lives
/// only while a sheet surface subscribes, so leaving the screen discards every
/// edit and a reopen reseeds from the stored sheet. [SheetEditController.save]
/// commits the changed editable delta and merges the derived cascade the API
/// returns, without reloading the sheet.
///
/// Rebuild granularity comes from `select`: field controls watch a single
/// dotted path of this state, so one edit rebuilds exactly one control and
/// the sheet screen itself never watches the state (AD-001).
library;

import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/sheet_defaults.dart';
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
}

/// Edit state of the sheet opened for one sheet id.
///
/// Rebuilds (and reseeds) whenever the open sheet or the template detail
/// changes: the loading state drops the value, which discards the edits, and a
/// successful load seeds from the sheet's stored data over the schema defaults
/// — the discard-confirmed refresh flow rides on that (GRN-17).
@riverpod
class SheetEditController extends _$SheetEditController {
  @override
  SheetEditState build(String sheetId) {
    final sheet = ref.watch(sheetOpenControllerProvider(sheetId)).sheet;
    if (sheet == null) return const SheetEditState();
    final template = ref
        .watch(templateDetailControllerProvider(sheet.templateId))
        .template;
    if (template == null) return const SheetEditState();
    final seed = seedFromSheet(template.fields, sheet.data);
    return SheetEditState(defaults: seed, data: seed);
  }

  /// Stores [value] at the dotted [path], keeping the previous state's maps
  /// untouched and recording the path for [SheetEditState.isPending].
  void setPath(String path, Object? value) {
    final data = _copyTree(state.data);
    setPathValue(data, path, value);
    state = SheetEditState(
      defaults: state.defaults,
      data: data,
      editedPaths: {...state.editedPaths, path},
      isSaving: state.isSaving,
      saveError: state.saveError,
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
    state = SheetEditState(
      defaults: state.defaults,
      data: data,
      editedPaths: {...state.editedPaths, ...paths},
      isSaving: state.isSaving,
      saveError: state.saveError,
    );
  }

  /// Commits the pending delta with one `PATCH /sheets/:id` and merges the
  /// derived cascade the API returns (SAV-08 … SAV-23).
  ///
  /// No-op while a request is in flight or the delta is empty, so Save and
  /// `Ctrl+S` can never double up and a clean sheet issues no request. On
  /// success the baseline advances to what the server now stores — the sent
  /// delta applied over the old baseline plus the cascade — so edits made
  /// while the request was in flight stay pending and only the sent fields
  /// clear their dirty flag. On failure every edit stays and the mapped copy
  /// is published for the footer; the PATCH is never retried automatically.
  Future<void> save() async {
    if (state.isSaving) return;
    final delta = state.pendingDelta;
    if (delta.isEmpty) return;
    state = SheetEditState(
      defaults: state.defaults,
      data: state.data,
      editedPaths: state.editedPaths,
      isSaving: true,
    );
    try {
      final cascade = await ref
          .read(apiClientProvider)
          .patchSheet(sheetId, delta);
      if (!ref.mounted) return;
      _applySaved(delta, cascade);
    } catch (error) {
      if (!ref.mounted) return;
      state = SheetEditState(
        defaults: state.defaults,
        data: state.data,
        editedPaths: state.editedPaths,
        saveError: failureCopy(error),
      );
    }
  }

  /// Merges the cascade into the working data and advances the baseline to the
  /// server state: the sent [delta] over the old baseline, then the [cascade].
  void _applySaved(
    Map<String, Object?> delta,
    Map<String, Object?> cascade,
  ) {
    final data = mergeCascade(state.data, cascade);
    final baseline = mergeCascade(
      mergeCascade(state.defaults, delta),
      cascade,
    );
    state = SheetEditState(
      defaults: baseline,
      data: data,
      editedPaths: state.editedPaths,
    );
  }
}

/// Deep copy of the nested value tree, sharing scalar leaves.
Map<String, Object?> _copyTree(Map<String, Object?> source) => {
  for (final entry in source.entries)
    entry.key: entry.value is Map<String, Object?>
        ? _copyTree(entry.value as Map<String, Object?>)
        : entry.value,
};
