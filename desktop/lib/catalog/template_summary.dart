/// Template summary derivation for the creation surface (TPL-02, TPL-03, TPL-05).
library;

import 'package:legnaris/api/models/template.dart';

/// One line of the template summary: a display name and the leaf fields it
/// contains, under their display names.
typedef FieldSummary = ({String name, List<String> members});

/// Reserved top-level groups that never reach the summary: internal switches
/// (`config`) and computed values (`derived`, `modifiers`). They carry no
/// decision value when creating a sheet; Phase 3 renders them in full.
const _hiddenGroups = {'config', 'derived', 'modifiers'};

/// Derives the summary of [fields]: one [FieldSummary] per visible top-level
/// key, in the order the API returned the keys (map insertion order).
///
/// A group lists every leaf beneath it recursively; a top-level leaf is its own
/// line with no members (TPL-03). Display names prefer the schema label and fall
/// back to a normalized form of the key, so no template prefix is hardcoded
/// (TPL-05). An empty [fields] tree yields an empty summary.
List<FieldSummary> summarizeFields(FieldGroup fields) => [
  for (final entry in fields.entries)
    if (!_hiddenGroups.contains(entry.key)) _summaryOf(entry.key, entry.value),
];

FieldSummary _summaryOf(String key, FieldNode node) {
  final schema = node.schema;
  if (schema != null) {
    return (name: _displayName(key, schema), members: const <String>[]);
  }
  return (
    name: _displayName(key, null),
    members: [
      for (final leaf in _leaves(node))
        _displayName(leaf.key, leaf.value.schema),
    ],
  );
}

Iterable<MapEntry<String, FieldNode>> _leaves(FieldNode node) sync* {
  final children = node.children;
  if (children == null) return;
  for (final child in children.entries) {
    if (child.value.schema != null) {
      yield child;
    } else {
      yield* _leaves(child.value);
    }
  }
}

String _displayName(String key, FieldSchema? schema) {
  final label = schema?.label.trim();
  if (label != null && label.isNotEmpty) return label;
  return _humanize(key);
}

String _humanize(String key) {
  final spaced = key
      .replaceAll(RegExp(r'[_-]+'), ' ')
      .replaceAllMapped(
        RegExp(r'([a-z0-9])([A-Z])'),
        (match) => '${match[1]} ${match[2]}',
      )
      .replaceAllMapped(
        RegExp(r'([A-Z]+)([A-Z][a-z])'),
        (match) => '${match[1]} ${match[2]}',
      )
      .trim();
  if (spaced.isEmpty) return key;
  final lower = spaced.toLowerCase();
  return lower[0].toUpperCase() + lower.substring(1);
}
