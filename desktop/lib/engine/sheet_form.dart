/// Composition of the sheet body (Tela 2): every section of the template's
/// field tree rendered as a labeled block of field rows (RND-01, RND-05,
/// RND-12, RND-16). Stateless; two control sources — the default controlled
/// path takes values from [SheetForm.data] and reports edits through
/// [SheetForm.onChanged], while [SheetForm.controlBuilder] hands each row's
/// control to the caller (the editable screen builds a per-field `select`
/// subscriber there, LGN-102).
library;

import 'package:flutter/material.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/theme/tokens.dart';

/// Builds the control widget of one field row, replacing the default
/// [SheetFieldControl]. The builder owns everything about that widget —
/// value subscription, edit reporting, sizing — while sections and labels
/// stay with [SheetForm].
typedef SheetControlBuilder = Widget Function(
  BuildContext context,
  SheetField field,
);

/// Builds the control widget of one exclusive row, replacing the default
/// [SheetExclusiveControl]. Injected together with [SheetForm.controlBuilder]
/// so the editable screen owns value subscription and edit reporting; a null
/// `onSelect` renders the control read-only.
typedef SheetExclusiveControlBuilder = Widget Function(
  BuildContext context,
  SheetExclusiveGroup group,
  String? selectedPath,
  ValueChanged<String>? onSelect,
);

/// Builds the preview-mark label of one row, given every dotted path the row
/// covers (a single leaf, or all members of an exclusive group). The returned
/// widget subscribes to the marks itself, so [SheetForm] stays stateless.
typedef SheetRowMarkBuilder = Widget Function(
  BuildContext context,
  List<String> paths,
);

/// Renders the whole sheet body from a template field tree and the current
/// values.
class SheetForm extends StatelessWidget {
  const SheetForm({
    super.key,
    required this.fields,
    this.data,
    this.onChanged,
    this.controlBuilder,
    this.exclusiveControlBuilder,
    this.rowMarkBuilder,
  }) : assert(
         (data == null) != (controlBuilder == null) &&
             (controlBuilder == null) == (exclusiveControlBuilder == null),
         'SheetForm needs exactly one control source: data, or both builders',
       );

  /// Template field tree; sections are derived from it.
  final FieldGroup fields;

  /// Current sheet values, resolved by dotted path; null in builder mode.
  final Map<String, Object?>? data;

  /// Reports an edit as `(path, value)`; null renders the controls inert.
  final void Function(String path, Object? value)? onChanged;

  /// Per-row control source; when set, [data] and [onChanged] are unused.
  final SheetControlBuilder? controlBuilder;

  /// Exclusive-row control source; provided together with [controlBuilder].
  final SheetExclusiveControlBuilder? exclusiveControlBuilder;

  /// Optional per-row preview mark source; when omitted the rows carry none.
  final SheetRowMarkBuilder? rowMarkBuilder;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final section in buildSections(fields))
          ..._section(context, section),
      ],
    );
  }

  List<Widget> _section(BuildContext context, SheetSection section) {
    final theme = Theme.of(context);
    final widgets = <Widget>[
      const SizedBox(height: AppSpacing.lg),
      Text(
        section.label,
        key: Key('sheet-section-${section.key}'),
        style: theme.textTheme.titleSmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
      const Divider(),
    ];
    // A run of consecutive table rows sharing the same columns draws one
    // header; any other row kind ends the run, so a later table starts a new
    // one. This keeps each table named once instead of repeating its header
    // on every row (RND-21).
    String? activeColumns;
    var tableIndex = 0;
    for (final row in section.rows) {
      if (row.exclusive != null) {
        widgets.add(_exclusiveRow(context, row.exclusive!));
        activeColumns = null;
      } else if (row.group != null) {
        final columns = _columnsKey(row.fields);
        if (columns != activeColumns) {
          widgets.add(
            _tableHeader(context, section.key, tableIndex, row.fields),
          );
        }
        widgets.add(_groupRow(context, row));
        activeColumns = columns;
        tableIndex++;
      } else {
        widgets.add(_row(context, row.fields.single));
        activeColumns = null;
      }
    }
    return widgets;
  }

  /// The caption row naming one table's columns: a blank first cell (the
  /// section title already names the group) and one short label per column,
  /// aligned like the cell below it (RND-21).
  Widget _tableHeader(
    BuildContext context,
    String sectionKey,
    int tableIndex,
    List<SheetField> fields,
  ) {
    final style = Theme.of(context).textTheme.bodySmall;
    return SizedBox(
      height: AppSizes.rowHeight,
      child: Row(
        key: Key('sheet-table-header-$sectionKey-$tableIndex'),
        children: [
          const Expanded(flex: 2, child: SizedBox.shrink()),
          for (final field in fields)
            _tableCell(
              field: field,
              header: true,
              child: Text(
                fieldColumnName(lastSegment(field.path), field.schema),
                style: style,
                textAlign: _cellAlignment(field.schema) == Alignment.centerRight
                    ? TextAlign.right
                    : TextAlign.left,
              ),
            ),
        ],
      ),
    );
  }

  /// One table row: the group's label in the first cell and one control per
  /// declared column, in order (RND-21).
  Widget _groupRow(BuildContext context, SheetRow row) {
    final theme = Theme.of(context);
    return SizedBox(
      height: AppSizes.rowHeight,
      child: Row(
        key: Key('sheet-group-row-${row.fields.first.path}'),
        children: [
          Expanded(
            flex: 2,
            child: _labelWithMark(
              context,
              row.group!.label,
              [for (final field in row.fields) field.path],
              theme.textTheme.bodyMedium,
            ),
          ),
          for (final field in row.fields)
            _tableCell(field: field, child: _cellControl(context, field)),
        ],
      ),
    );
  }

  /// One table cell: an equal flex per column, its content aligned right for a
  /// numeric leaf and left otherwise, so numbers line up under their headers.
  /// Data cells carry a key for tests; header cells do not, so the key is
  /// unique to the row it describes.
  Widget _tableCell({
    required SheetField field,
    required Widget child,
    bool header = false,
  }) {
    final alignment = _cellAlignment(field.schema);
    return Expanded(
      flex: 3,
      child: Padding(
        padding: const EdgeInsets.only(left: AppSpacing.sm),
        child: Align(
          key: header ? null : Key('sheet-table-cell-${field.path}'),
          alignment: alignment,
          child: child,
        ),
      ),
    );
  }

  /// The control of one field, shared by plain rows and table cells: the
  /// injected builder when present, the data-driven control otherwise.
  Widget _cellControl(BuildContext context, SheetField field) {
    if (controlBuilder != null) return controlBuilder!(context, field);
    return SheetFieldControl(
      path: field.path,
      schema: field.schema,
      value: valueAtPath(data!, field.path),
      onChanged: onChanged == null
          ? null
          : (value) => onChanged!(field.path, value),
    );
  }

  Widget _row(BuildContext context, SheetField field) {
    final theme = Theme.of(context);
    final label = fieldDisplayName(field.path.split('.').last, field.schema);
    return Padding(
      key: Key('sheet-row-${field.path}'),
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
      child: Row(
        children: [
          Expanded(
            child: _labelWithMark(
              context,
              label,
              [field.path],
              theme.textTheme.bodyMedium,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          _cellControl(context, field),
        ],
      ),
    );
  }

  /// One exclusive group row: the humanized group id as the label and the
  /// group's single control. Its members never render as independent per-field
  /// controls (EXC-09), so an exclusive row can never expose two toggles.
  Widget _exclusiveRow(BuildContext context, SheetExclusiveGroup group) {
    final theme = Theme.of(context);
    final label = fieldDisplayName(group.id, null);
    final selected = data == null ? null : _activeOption(group);
    return Padding(
      key: Key('sheet-row-${group.id}'),
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            flex: 2,
            child: _labelWithMark(
              context,
              label,
              [for (final option in group.options) option.path],
              theme.textTheme.bodyMedium,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            flex: 3,
            child:
                exclusiveControlBuilder != null
                    ? exclusiveControlBuilder!(context, group, selected, null)
                    : SheetExclusiveControl(
                      group: group,
                      selectedPath: selected,
                      onSelect: null,
                    ),
          ),
        ],
      ),
    );
  }

  /// The active option of [group] for data mode: the first member whose value
  /// counts as on, or null when the group has no active member (EXC-11).
  String? _activeOption(SheetExclusiveGroup group) {
    for (final option in group.options) {
      if (exclusiveValueIsOn(valueAtPath(data!, option.path))) {
        return option.path;
      }
    }
    return null;
  }

  /// One row's label plus, when a [rowMarkBuilder] is injected, its preview
  /// mark. The builder's widget owns its own subscription, so this stays a
  /// stateless composition.
  Widget _labelWithMark(
    BuildContext context,
    String text,
    List<String> paths,
    TextStyle? style,
  ) {
    final builder = rowMarkBuilder;
    if (builder == null) return Text(text, style: style);
    return Row(
      children: [
        Flexible(
          child: Text(text, style: style, overflow: TextOverflow.ellipsis),
        ),
        const SizedBox(width: AppSpacing.sm),
        builder(context, paths),
      ],
    );
  }
}

/// The column identity of a table row: its ordered header labels. Consecutive
/// rows with the same identity share one header (RND-21).
String _columnsKey(List<SheetField> fields) => [
  for (final field in fields)
    fieldColumnName(lastSegment(field.path), field.schema),
].join('\u0000');

/// The alignment of a table cell: numbers right, everything else left.
Alignment _cellAlignment(FieldSchema schema) =>
    schema.type == 'number' ? Alignment.centerRight : Alignment.centerLeft;
