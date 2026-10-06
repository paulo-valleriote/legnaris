/// Section derivation for the schema-driven sheet renderer (RND-01 … RND-05):
/// one section per top-level field key, with every leaf beneath it flattened
/// into dotted paths and grouped into render rows — stacked leaves, exclusive
/// groups, or table rows declared by the template. Pure derivation — no
/// widgets, no state, no per-system literals.
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
/// metadata, replacing the members' independent controls (EXC-09); a table row
/// carries the group's [group] label and one leaf per declared column, replacing
/// the children's stacked rows (RND-21).
typedef SheetRow = ({
  List<SheetField> fields,
  SheetExclusiveGroup? exclusive,
  ({String label})? group,
});

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
/// single-field section (RND-02, RND-03). [SheetSection.leaves] always carries
/// the full flattened set, while [SheetSection.rows] may collapse a declared
/// group into one table row (RND-21). Labels come from the field tree itself,
/// never from a per-system literal (RND-04).
List<SheetSection> buildSections(FieldGroup fields) {
  final sections = <SheetSection>[];
  for (final entry in fields.entries) {
    final leaves = _flatten(entry.key, entry.value);
    if (leaves.isEmpty) continue;
    sections.add((
      key: entry.key,
      label: fieldDisplayName(entry.key, entry.value.schema),
      leaves: leaves,
      rows: _buildRows(entry.key, entry.value),
    ));
  }
  return sections;
}

/// Collapses one section's [root] tree into render rows.
///
/// A leaf child becomes its own normal row; a group child that declares
/// [FieldNode.columns] becomes one table row holding the named leaves in
/// declared order (an unknown or group column is skipped; when no declared
/// column resolves, the group falls back to stacked rows); every other group
/// child is flattened as before. Exclusive grouping still applies only to leaf
/// rows: every leaf sharing a non-empty `exclusiveGroup` joins one exclusive
/// row placed at the first member's position, regardless of adjacency, and a
/// single-member group is not collapsed (EXC-09, EXC-12). Grouping by id (not
/// by adjacency) matches the backend, which groups by id across the whole
/// parent.
List<SheetRow> _buildRows(String prefix, FieldNode root) {
  final rows = <SheetRow>[];
  final groupRowIndex = <String, int>{};
  final groupOptions = <String, List<SheetField>>{};

  void addLeaf(SheetField leaf) {
    final id = leaf.schema.exclusiveGroup;
    if (id == null || id.isEmpty) {
      rows.add((fields: [leaf], exclusive: null, group: null));
      return;
    }
    final existing = groupRowIndex[id];
    if (existing == null) {
      final options = <SheetField>[leaf];
      groupRowIndex[id] = rows.length;
      groupOptions[id] = options;
      rows.add((fields: options, exclusive: null, group: null));
      return;
    }
    groupOptions[id]!.add(leaf);
  }

  void walk(String path, FieldNode node) {
    final schema = node.schema;
    if (schema != null) {
      addLeaf((path: path, schema: schema));
      return;
    }
    final children = node.children;
    if (children == null) return;

    final columns = node.columns;
    if (columns != null && columns.isNotEmpty) {
      final tableFields = <SheetField>[];
      for (final key in columns) {
        final columnSchema = children[key]?.schema;
        if (columnSchema == null) continue;
        tableFields.add((path: '$path.$key', schema: columnSchema));
      }
      if (tableFields.isNotEmpty) {
        rows.add((
          fields: tableFields,
          exclusive: null,
          group: (label: groupDisplayName(lastSegment(path), node.label)),
        ));
        return;
      }
    }

    for (final child in children.entries) {
      walk('$path.${child.key}', child.value);
    }
  }

  walk(prefix, root);

  for (final entry in groupOptions.entries) {
    final options = entry.value;
    if (options.length < 2) continue;
    rows[groupRowIndex[entry.key]!] = (
      fields: options,
      exclusive: (id: entry.key, options: options),
      group: null,
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

/// The display name of a group node: its declared [label] when non-empty,
/// otherwise a normalized form of the key (mirrors [fieldDisplayName]).
String groupDisplayName(String key, String? label) {
  final trimmed = label?.trim();
  if (trimmed != null && trimmed.isNotEmpty) return trimmed;
  return _humanize(key);
}

/// The short header of a leaf rendered as a table column: its `column_label`
/// when non-empty, otherwise the [fieldDisplayName] fallback (label, then
/// humanized key).
String fieldColumnName(String key, FieldSchema schema) {
  final column = schema.columnLabel?.trim();
  if (column != null && column.isNotEmpty) return column;
  return fieldDisplayName(key, schema);
}

/// The segment after the last dot of a dotted field [path] — the key the
/// display-name fallbacks (and table column names) work on.
String lastSegment(String path) {
  final index = path.lastIndexOf('.');
  return index < 0 ? path : path.substring(index + 1);
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
