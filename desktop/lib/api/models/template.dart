/// Template models decoded from the backend `TemplateDTO` contract
/// (FE1-08, FE1-09).
library;

/// Field tree of a template: field name → [FieldNode].
typedef FieldGroup = Map<String, FieldNode>;

/// One node of the template field tree: a leaf exposes only `schema`, a group
/// exposes only `children` (FE1-08).
class FieldNode {
  const FieldNode({this.schema, this.children});

  factory FieldNode.fromJson(Map<String, Object?> json) {
    final schemaJson = json['schema'];
    final childrenJson = json['children'];
    return FieldNode(
      schema:
          schemaJson == null
              ? null
              : FieldSchema.fromJson(schemaJson as Map<String, Object?>),
      children:
          childrenJson == null
              ? null
              : _fieldGroupFromJson(childrenJson as Map<String, Object?>),
    );
  }

  /// Leaf schema, present on leaves and null on groups.
  final FieldSchema? schema;

  /// Child nodes, present on groups and null on leaves.
  final FieldGroup? children;
}

/// Definition of a single field (FE1-09). [type] is an opaque string: mapping
/// it to widgets is Phase 3 (UI Engine), out of scope here.
class FieldSchema {
  const FieldSchema({
    required this.type,
    required this.label,
    this.formula,
    this.dependsOn,
    required this.isEditable,
    required this.defaultValue,
  });

  factory FieldSchema.fromJson(Map<String, Object?> json) => FieldSchema(
    type: json['type'] as String,
    label: json['label'] as String,
    formula: json['formula'] as String?,
    dependsOn:
        (json['depends_on'] as List<Object?>?)
            ?.map((path) => List<String>.from(path as List<Object?>))
            .toList(),
    isEditable: json['is_editable'] as bool,
    defaultValue: (json['default'] as num).toDouble(),
  );

  /// Opaque field type (e.g. `number`), rendered by the UI Engine in Phase 3.
  final String type;

  /// Human-readable field label.
  final String label;

  /// Formula of a derived field; null when the field has none.
  final String? formula;

  /// Dotted-path dependencies of [formula]; null when omitted in the JSON.
  final List<List<String>>? dependsOn;

  /// Whether the user may edit this field directly.
  final bool isEditable;

  /// Initial value of the field; explicit mapping of the reserved JSON key
  /// `default`.
  final double defaultValue;
}

/// A template as returned by `GET /templates/:id`: id, name, default flag and
/// the field tree (FE1-08).
class TemplateDetail {
  const TemplateDetail({
    required this.id,
    required this.name,
    required this.isDefault,
    required this.fields,
  });

  factory TemplateDetail.fromJson(Map<String, Object?> json) => TemplateDetail(
    id: json['id'] as String,
    name: json['name'] as String,
    isDefault: json['default'] as bool,
    fields: _fieldGroupFromJson(json['fields'] as Map<String, Object?>),
  );

  /// Decodes the list payload of `GET /templates`, where every item is the
  /// same TemplateDTO as the detail (confirmed spec assumption).
  static List<TemplateCatalog> listFromJson(Object? json) =>
      (json as List<Object?>)
          .map(
            (item) => TemplateCatalog.fromJson(item as Map<String, Object?>),
          )
          .toList();

  /// Template identifier, e.g. `fantasy_v1`.
  final String id;

  /// Human-readable template name.
  final String name;

  /// Whether this is the default template; explicit mapping of the reserved
  /// JSON key `default`.
  final bool isDefault;

  /// Field tree of the template.
  final FieldGroup fields;
}

/// Catalog entry of `GET /templates`.
///
/// The backend serves the same `TemplateDTO` for catalog and detail, so both
/// endpoints share one model (confirmed spec assumption: avoid a second
/// model); the Phase 2 catalog UI ignores [TemplateDetail.fields] in the list.
typedef TemplateCatalog = TemplateDetail;

FieldGroup _fieldGroupFromJson(Map<String, Object?> json) => json.map(
  (name, node) =>
      MapEntry(name, FieldNode.fromJson(node as Map<String, Object?>)),
);
