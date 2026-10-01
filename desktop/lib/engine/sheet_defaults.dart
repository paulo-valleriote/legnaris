/// Shared derivation of a sheet draft from a template's field tree: every leaf
/// seeded with its schema default, addressed by dotted path. Used by both the
/// editable sheet screen (Tela 2) and the read-only creation preview (LGN-100),
/// so the two cannot drift.
library;

import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/field_grouping.dart';

/// Builds the initial values of [fields]: every leaf gets its seed value
/// under its dotted path, in section order. Text fields start empty and
/// booleans start false, mirroring the backend's `SeedValue` — the JSON
/// `default` key stays numeric for every type, so a text field must not be
/// seeded with its float default.
Map<String, Object?> seedDefaults(FieldGroup fields) {
  final data = <String, Object?>{};
  for (final section in buildSections(fields)) {
    for (final field in section.leaves) {
      setPathValue(data, field.path, seedValue(field.schema));
    }
  }
  return data;
}

/// The initial value of one field: empty for text, false for booleans, and
/// the numeric default for everything else.
Object? seedValue(FieldSchema schema) {
  final type = schema.type;
  if (type == 'text' || type == 'string') {
    return '';
  }
  if (type == 'bool') {
    return false;
  }
  return schema.defaultValue;
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
