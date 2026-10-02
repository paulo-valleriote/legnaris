/// In-memory edit state for one open sheet (LGN-102, GRN-01 … GRN-06).
///
/// The family is keyed by template id and autoDispose: the state lives only
/// while a sheet surface subscribes, so leaving the screen discards every
/// edit and a reopen reseeds from the schema defaults. Nothing here is
/// persisted or sent to the API — the save cycle is Phase 4.
///
/// Rebuild granularity comes from `select`: field controls watch a single
/// dotted path of this state, so one edit rebuilds exactly one control and
/// the sheet screen itself never watches the state (AD-001).
library;

import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/engine/sheet_defaults.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sheet_edit_provider.g.dart';

/// Immutable values of one sheet's edit state, addressed by dotted path.
class SheetEditState {
  const SheetEditState({
    this.defaults = const {},
    this.data = const {},
    this.editedPaths = const {},
  });

  /// Schema defaults the state was seeded from; the reference point for
  /// [isPending].
  final Map<String, Object?> defaults;

  /// Current values. State maps are never mutated in place — every write
  /// produces a new tree (copy-on-write), so listeners can compare states.
  final Map<String, Object?> data;

  /// Paths written at least once since the seed. A path back at its default
  /// value no longer counts as pending.
  final Set<String> editedPaths;

  /// True while any edited path differs from its seeded default: the marker
  /// and both discard guards key off this (GRN-11 … GRN-19).
  bool get isPending => editedPaths.any(
    (path) => valueAtPath(data, path) != valueAtPath(defaults, path),
  );
}

/// Edit state of the sheet for one template id.
///
/// Rebuilds (and reseeds) whenever the template detail state changes: the
/// loading state drops the template, which discards the edits, and a
/// successful reload seeds the fresh schema — the discard-confirmed refresh
/// flow rides on that (GRN-17).
@riverpod
class SheetEditController extends _$SheetEditController {
  @override
  SheetEditState build(String templateId) {
    final template = ref
        .watch(templateDetailControllerProvider(templateId))
        .template;
    if (template == null) return const SheetEditState();
    final defaults = seedDefaults(template.fields);
    return SheetEditState(defaults: defaults, data: defaults);
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
