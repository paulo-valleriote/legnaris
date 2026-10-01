/// Sheet model decoded from the backend `SheetDTO` contract (FE1-10) with the
/// dotted-path accessor used by the UI Engine (FE1-12 … FE1-14).
library;

/// Thrown when a dotted path is malformed: empty path or empty segment
/// (FE1-14). Distinct from a missing value, which resolves to null.
class InvalidPathException implements Exception {
  InvalidPathException(this.message);

  final String message;

  @override
  String toString() => 'InvalidPathException: $message';
}

/// A sheet (ficha): identity, owning template, timestamps and the dynamic
/// `data` payload.
class Sheet {
  const Sheet({
    required this.id,
    required this.templateId,
    required this.name,
    required this.data,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Sheet.fromJson(Map<String, Object?> json) => Sheet(
    id: json['id'] as String,
    templateId: json['template_id'] as String,
    name: json['name'] as String,
    data: json['data'] as Map<String, Object?>,
    createdAt: DateTime.parse(json['created_at'] as String),
    updatedAt: DateTime.parse(json['updated_at'] as String),
  );

  final String id;

  /// Identifier of the template this sheet was created from (JSON key
  /// `template_id`).
  final String templateId;

  final String name;

  /// Dynamic cell values, nested as decoded from JSON; numbers keep the exact
  /// int/double type they arrive with (FE1-11).
  final Map<String, Object?> data;

  final DateTime createdAt;

  final DateTime updatedAt;

  /// Resolves the dotted [path] (e.g. `stats.dexterity`) against [data].
  /// Delegates to [valueAtPath] so the traversal lives in one place.
  Object? valueAt(String path) => valueAtPath(data, path);
}

/// Resolves the dotted [path] (e.g. `stats.dexterity`) against [data].
///
/// Returns the value the path points at (FE1-12). A missing segment or an
/// intermediate node that is not a map resolves to null and never throws
/// (FE1-13). An empty path or an empty segment throws [InvalidPathException]
/// (FE1-14) — a malformed path is a bug, not missing data.
Object? valueAtPath(Map<String, Object?> data, String path) {
  if (path.isEmpty) {
    throw InvalidPathException('path must not be empty');
  }

  Object? current = data;
  for (final segment in path.split('.')) {
    if (segment.isEmpty) {
      throw InvalidPathException('path "$path" has an empty segment');
    }
    if (current is! Map<String, Object?>) {
      return null;
    }
    current = current[segment];
  }
  return current;
}
