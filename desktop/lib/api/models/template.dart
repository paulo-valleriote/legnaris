/// Template models decoded from the backend `TemplateDTO` contract
/// (FE1-08, FE1-09).
library;

/// Field tree of a template: field name → [FieldNode].
typedef FieldGroup = Map<String, FieldNode>;

/// One node of the template field tree: a leaf exposes only `schema`, a group
/// exposes `children` plus its optional table metadata `label`/`columns`
/// (FE1-08, RND-21).
class FieldNode {
  const FieldNode({this.schema, this.children, this.label, this.columns});

  factory FieldNode.fromJson(Map<String, Object?> json) {
    final schemaJson = json['schema'];
    final childrenJson = json['children'];
    final columnsJson = json['columns'];
    return FieldNode(
      schema:
          schemaJson == null
              ? null
              : FieldSchema.fromJson(schemaJson as Map<String, Object?>),
      children:
          childrenJson == null
              ? null
              : _fieldGroupFromJson(childrenJson as Map<String, Object?>),
      label: json['label'] as String?,
      columns:
          columnsJson == null
              ? null
              : List<String>.from(columnsJson as List<Object?>),
    );
  }

  /// Leaf schema, present on leaves and null on groups.
  final FieldSchema? schema;

  /// Child nodes, present on groups and null on leaves.
  final FieldGroup? children;

  /// Display name of a group node (table row header or section); null when the
  /// key does not declare one, falling back to the humanized key.
  final String? label;

  /// Child keys rendered as ONE table row, in column order; null or empty
  /// renders the children as stacked rows.
  final List<String>? columns;
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
    this.exclusiveGroup,
    this.optionLabel,
    this.columnLabel,
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
    exclusiveGroup: json['exclusive_group'] as String?,
    optionLabel: json['option_label'] as String?,
    columnLabel: json['column_label'] as String?,
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

  /// Mutually exclusive group this leaf belongs to (JSON key
  /// `exclusive_group`); null when the key is absent or null. Leaves sharing a
  /// non-empty group under the same parent may have at most one non-zero value.
  final String? exclusiveGroup;

  /// Short label used when this leaf is shown as an option of its exclusive
  /// group (JSON key `option_label`); null when the key is absent or null.
  /// Empty falls back to [label].
  final String? optionLabel;

  /// Short label used when this leaf is rendered as a table column (JSON key
  /// `column_label`); null when the key is absent or null. Empty falls back to
  /// [label], then to the humanized key.
  final String? columnLabel;
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
