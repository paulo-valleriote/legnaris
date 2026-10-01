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
    return [
      const SizedBox(height: AppSpacing.lg),
      Text(
        section.label,
        key: Key('sheet-section-${section.key}'),
        style: theme.textTheme.titleSmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
      const Divider(),
      for (final row in section.rows)
        if (row.exclusive != null)
          _exclusiveRow(context, row.exclusive!)
        else
          _row(context, row.fields.single),
    ];
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
        children: [
          Expanded(child: Text(label, style: theme.textTheme.bodyMedium)),
          const SizedBox(width: AppSpacing.md),
          if (exclusiveControlBuilder != null)
            exclusiveControlBuilder!(context, group, selected, null)
          else
            SheetExclusiveControl(
              group: group,
              selectedPath: selected,
              onSelect: null,
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

  Widget _row(BuildContext context, SheetField field) {
    final theme = Theme.of(context);
    final label = fieldDisplayName(field.path.split('.').last, field.schema);
    return Padding(
      key: Key('sheet-row-${field.path}'),
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
      child: Row(
        children: [
          Expanded(child: Text(label, style: theme.textTheme.bodyMedium)),
          const SizedBox(width: AppSpacing.md),
          if (controlBuilder != null)
            controlBuilder!(context, field)
          else
            SheetFieldControl(
              path: field.path,
              schema: field.schema,
              value: valueAtPath(data!, field.path),
              onChanged: onChanged == null
                  ? null
                  : (value) => onChanged!(field.path, value),
            ),
        ],
      ),
    );
  }
}
