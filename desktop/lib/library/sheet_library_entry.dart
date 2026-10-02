/// One entry in the local sheet library index: the four persisted fields.
library;

/// A sheet index entry, the lean copy of a sheet's identity with no `data`
/// payload (LIB-02, LIB-08).
class SheetLibraryEntry {
  const SheetLibraryEntry({
    required this.id,
    required this.name,
    required this.templateId,
    required this.createdAt,
  });

  /// Reads the persisted snake_case keys; a missing or wrong-typed field is a
  /// [FormatException] so a corrupt store is detected rather than misread.
  factory SheetLibraryEntry.fromJson(Map<String, Object?> json) {
    final id = json['id'];
    final name = json['name'];
    final templateId = json['template_id'];
    final createdAt = json['created_at'];
    if (id is! String ||
        name is! String ||
        templateId is! String ||
        createdAt is! String) {
      throw const FormatException(
        'SheetLibraryEntry requires string id, name, template_id and created_at',
      );
    }
    return SheetLibraryEntry(
      id: id,
      name: name,
      templateId: templateId,
      createdAt: DateTime.parse(createdAt),
    );
  }

  final String id;

  final String name;

  /// Identifier of the template this sheet was created from (JSON key
  /// `template_id`).
  final String templateId;

  /// Creation timestamp taken verbatim from the API sheet (LIB-03).
  final DateTime createdAt;

  /// Whether the entry carries the identity fields the store requires; an
  /// empty id, name or template id must never be persisted (LIB-20).
  bool get isValid => id.isNotEmpty && name.isNotEmpty && templateId.isNotEmpty;

  /// Writes the four persisted keys with `created_at` as ISO-8601 UTC; the
  /// `data` payload is never part of this model (LIB-08).
  Map<String, Object?> toJson() => {
    'id': id,
    'name': name,
    'template_id': templateId,
    'created_at': createdAt.toUtc().toIso8601String(),
  };
}
