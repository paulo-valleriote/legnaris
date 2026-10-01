/// Shared derivation of a sheet draft from a template's field tree: every leaf
/// seeded with its schema default, addressed by dotted path. Used by both the
/// editable sheet screen (Tela 2) and the read-only creation preview (LGN-100),
/// so the two cannot drift.
library;

import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/field_grouping.dart';

/// Builds the initial values of [fields]: every leaf gets its
/// [FieldSchema.defaultValue] under its dotted path, in section order.
Map<String, Object?> seedDefaults(FieldGroup fields) {
  final data = <String, Object?>{};
  for (final section in buildSections(fields)) {
    for (final field in section.leaves) {
      setPathValue(data, field.path, field.schema.defaultValue);
    }
  }
  return data;
}

/// Sets [value] at the dotted [path] of [data], creating the intermediate maps
/// the path needs and overwriting the leaf when it already exists.
void setPathValue(Map<String, Object?> data, String path, Object? value) {
  final segments = path.split('.');
  Map<String, Object?> cursor = data;
  for (final segment in segments.take(segments.length - 1)) {
    final next = cursor[segment];
    if (next is Map<String, Object?>) {
      cursor = next;
    } else {
      final created = <String, Object?>{};
      cursor[segment] = created;
      cursor = created;
    }
  }
  cursor[segments.last] = value;
}
