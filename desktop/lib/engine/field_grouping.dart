/// Section derivation for the schema-driven sheet renderer (RND-01 … RND-05):
/// one section per top-level field key, with every leaf beneath it flattened
/// into dotted paths. Pure derivation — no widgets, no state, no per-system
/// literals.
library;

import 'package:legnaris/api/models/template.dart';

/// One leaf field of a section: its dotted path (`stats.dexterity`) and the
/// schema that decides how it is rendered.
typedef SheetField = ({String path, FieldSchema schema});

/// One mutually exclusive set inside a section: the group id advertised by the
/// contract and its member leaves, in tree/API order (EXC-09).
typedef SheetExclusiveGroup = ({String id, List<SheetField> options});

/// One render row of a section. A normal row carries a single untagged leaf
/// and no group; an exclusive row carries every member of one group plus its
/// metadata, replacing the members' independent controls (EXC-09).
typedef SheetRow = ({List<SheetField> fields, SheetExclusiveGroup? exclusive});

/// One top-level section: the source key, its display label, every leaf
/// beneath it in tree order, and the render rows derived from those leaves.
typedef SheetSection = ({
  String key,
  String label,
  List<SheetField> leaves,
  List<SheetRow> rows,
});

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
      rows: _buildRows(leaves),
    ));
  }
  return sections;
}

/// Collapses the flattened [leaves] of one section into render rows: every leaf
/// sharing a non-empty `exclusiveGroup` joins one exclusive row placed at the
/// first member's position, regardless of adjacency; every other leaf stays its
/// own normal row. A single-member group is not collapsed (EXC-12). Grouping by
/// id (not by adjacency) matches the backend, which groups by id across the
/// whole parent.
List<SheetRow> _buildRows(List<SheetField> leaves) {
  final rows = <SheetRow>[];
  final groupRowIndex = <String, int>{};
  final groupOptions = <String, List<SheetField>>{};

  for (final leaf in leaves) {
    final id = leaf.schema.exclusiveGroup;
    if (id == null || id.isEmpty) {
      rows.add((fields: [leaf], exclusive: null));
      continue;
    }
    final existing = groupRowIndex[id];
    if (existing == null) {
      final options = <SheetField>[leaf];
      groupRowIndex[id] = rows.length;
      groupOptions[id] = options;
      rows.add((fields: options, exclusive: null));
      continue;
    }
    groupOptions[id]!.add(leaf);
  }

  for (final entry in groupOptions.entries) {
    final options = entry.value;
    if (options.length < 2) continue;
    rows[groupRowIndex[entry.key]!] = (
      fields: options,
      exclusive: (id: entry.key, options: options),
    );
  }
  return rows;
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
