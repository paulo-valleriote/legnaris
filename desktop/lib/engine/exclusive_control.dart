/// Renders one mutually exclusive group as a single dense dropdown
/// (EXC-09 … EXC-11, EXC-27).
///
/// Every group uses the same control: a `DropdownButton` whose entries are the
/// group's members, each labelled by its short `option_label` when present and
/// by its schema label otherwise. A group with no active default offers a
/// `None` entry that clears the group; a group with a default does not. When a
/// default-active group has no active member (data seeded before the group
/// existed, or every member explicitly zeroed), the dropdown shows the `None`
/// label as its hint so the row never renders blank. A null
/// [SheetExclusiveControl.onSelect] keeps the dropdown disabled, so read-only
/// surfaces show the active option (or `None`) and nothing selectable.
library;

import 'package:flutter/material.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_mark.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// Whether an exclusive option value counts as active: any non-zero `num` for
/// the 0/1 proxy leaves, off otherwise. The backend contract is numeric 0/1
/// proxies and rejects a non-numeric member with a 400, so a bool is off.
bool exclusiveValueIsOn(Object? value) {
  if (value is num) return value != 0;
  return false;
}

/// Sentinel the dropdown reports when its `None` entry is selected. The caller
/// maps it to a null selection (clear the whole group); a real dotted member
/// path can never collide with it.
const String exclusiveNoneValue = '__none__';

/// One exclusive group rendered as a single dropdown.
class SheetExclusiveControl extends StatelessWidget {
  const SheetExclusiveControl({
    super.key,
    required this.group,
    this.selectedPath,
    this.mark,
    this.onSelect,
  });

  /// The group to render: its id and its member leaves in API order.
  final SheetExclusiveGroup group;

  /// Dotted path of the active option; null when no option is active.
  final String? selectedPath;

  /// Preview marking of the last dry-run for any member of this group, or
  /// null. Drives the change tint on the dropdown.
  final SheetMark? mark;

  /// Reports a newly selected option path (or [exclusiveNoneValue]); null
  /// renders the dropdown disabled.
  final ValueChanged<String>? onSelect;

  /// Whether the group offers a `None` entry: only when no member carries a
  /// non-zero default, so a group with an active default always has a value.
  bool get _allowsNone =>
      group.options.every((option) => option.schema.defaultValue == 0);

  /// The dropdown value: the active member, the `None` sentinel when nothing is
  /// active and the group allows it, or null when nothing can be shown.
  String? get _value {
    final selected = selectedPath;
    if (selected != null && group.options.any((o) => o.path == selected)) {
      return selected;
    }
    return _allowsNone ? exclusiveNoneValue : null;
  }

  /// The option text: the member's short `option_label` when non-empty, else
  /// its schema label (or the humanized key when the label is empty).
  String _optionLabel(SheetField option) {
    final label = option.schema.optionLabel?.trim();
    if (label != null && label.isNotEmpty) return label;
    return fieldDisplayName(option.path.split('.').last, option.schema);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final tint = mark == null ? null : sheetMarkTint(theme.colorScheme);
    final dropdown = DropdownButton<String>(
      key: Key('sheet-exclusive-${group.id}'),
      value: _value,
      hint: _value == null
          ? Text(
              localizations.fieldNone,
              key: const Key('sheet-exclusive-hint-none'),
              style: theme.textTheme.bodyMedium,
            )
          : null,
      isDense: true,
      isExpanded: true,
      borderRadius: BorderRadius.circular(AppRadius.control),
      style: theme.textTheme.bodyMedium,
      iconSize: AppSizes.emphasis,
      onChanged: onSelect == null
          ? null
          : (value) {
              if (value != null) onSelect!(value);
            },
      items: [
        if (_allowsNone)
          DropdownMenuItem<String>(
            value: exclusiveNoneValue,
            child: Text(
              localizations.fieldNone,
              key: const Key('sheet-exclusive-option-none'),
              style: theme.textTheme.bodyMedium,
            ),
          ),
        for (final option in group.options)
          DropdownMenuItem<String>(
            value: option.path,
            child: Text(
              _optionLabel(option),
              key: Key('sheet-exclusive-option-${option.path}'),
              style: theme.textTheme.bodyMedium,
              overflow: TextOverflow.ellipsis,
            ),
          ),
      ],
    );
    return SizedBox(
      width: AppSizes.fieldControlWidth,
      height: AppSizes.controlHeight,
      child: tint == null
          ? dropdown
          : DecoratedBox(
              key: Key('sheet-mark-tint-${group.id}'),
              decoration: BoxDecoration(
                color: tint,
                borderRadius: BorderRadius.circular(AppRadius.control),
              ),
              child: dropdown,
            ),
    );
  }
}
