/// Section derivation for the schema-driven sheet renderer (RND-01 … RND-05):
/// one section per top-level field key, with every leaf beneath it flattened
/// into dotted paths. Pure derivation — no widgets, no state, no per-system
/// literals.
library;

import 'package:legnaris/api/models/template.dart';

/// One leaf field of a section: its dotted path (`stats.dexterity`) and the
/// schema that decides how it is rendered.
typedef SheetField = ({String path, FieldSchema schema});

/// One top-level section: the source key, its display label and every leaf
/// beneath it, in tree order.
typedef SheetSection = ({String key, String label, List<SheetField> leaves});

/// Derives the sections of [fields]: one section per top-level key, in the
/// order the API returned the keys (map insertion order).
///
/// A group is flattened recursively into its leaves; a group with no leaves is
/// omitted because it has nothing to render; a top-level leaf becomes its own
/// single-field section (RND-02, RND-03). Labels come from the field tree
/// itself, never from a per-system literal (RND-04).
List<SheetSection> buildSections(FieldGroup fields) {
  final sections = <SheetSection>[];
  for (final entry in fields.entries) {
    final leaves = _flatten(entry.key, entry.value);
    if (leaves.isEmpty) continue;
    sections.add((
      key: entry.key,
      label: fieldDisplayName(entry.key, entry.value.schema),
      leaves: leaves,
    ));
  }
  return sections;
}

/// The display name of a field: the schema label when it is non-empty,
/// otherwise a normalized form of the key. Shared with the Phase 2 template
/// summary so both derive names the same way (RND-05).
String fieldDisplayName(String key, FieldSchema? schema) {
  final label = schema?.label.trim();
  if (label != null && label.isNotEmpty) return label;
  return _humanize(key);
}

List<SheetField> _flatten(String prefix, FieldNode node) {
  final schema = node.schema;
  if (schema != null) {
    return [(path: prefix, schema: schema)];
  }
  final children = node.children;
  if (children == null) return const [];
  return [
    for (final child in children.entries)
      ..._flatten('$prefix.${child.key}', child.value),
  ];
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
