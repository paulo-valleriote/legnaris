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

/// Renders the whole sheet body from a template field tree and the current
/// values.
class SheetForm extends StatelessWidget {
  const SheetForm({
    super.key,
    required this.fields,
    this.data,
    this.onChanged,
    this.controlBuilder,
  }) : assert(
         (data == null) != (controlBuilder == null),
         'SheetForm needs exactly one control source: data or controlBuilder',
       );

  /// Template field tree; sections are derived from it.
  final FieldGroup fields;

  /// Current sheet values, resolved by dotted path; null in builder mode.
  final Map<String, Object?>? data;

  /// Reports an edit as `(path, value)`; null renders the controls inert.
  final void Function(String path, Object? value)? onChanged;

  /// Per-row control source; when set, [data] and [onChanged] are unused.
  final SheetControlBuilder? controlBuilder;

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
      for (final field in section.leaves) _row(context, field),
    ];
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
