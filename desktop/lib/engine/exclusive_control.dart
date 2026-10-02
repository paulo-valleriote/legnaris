/// Renders one mutually exclusive group as a single control (EXC-09 … EXC-11).
///
/// A group of at most three options whose labels all fit the segmented limit is
/// drawn as a [SegmentedButton]; a longer option list or a longer label falls
/// back to a vertical radio group. A null [SheetExclusiveControl.onSelect]
/// keeps the same control shape but disables it, so read-only surfaces show the
/// active option selected and nothing selectable.
library;

import 'package:flutter/material.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/theme/tokens.dart';

/// Whether an exclusive option value counts as active: `true` for bools, any
/// non-zero `num` for the 0/1 proxy leaves, off otherwise.
///
/// The backend contract is numeric 0/1 proxies and rejects a non-numeric member
/// with a 400, so the `bool` branch only covers stale or foreign data; it stays
/// defensive rather than being the primary path.
bool exclusiveValueIsOn(Object? value) {
  if (value is bool) return value;
  if (value is num) return value != 0;
  return false;
}

/// One exclusive group rendered as a segmented selector or a radio fallback.
class SheetExclusiveControl extends StatelessWidget {
  const SheetExclusiveControl({
    super.key,
    required this.group,
    this.selectedPath,
    this.onSelect,
  });

  /// The group to render: its id and its member leaves in API order.
  final SheetExclusiveGroup group;

  /// Dotted path of the active option; null when no option is active.
  final String? selectedPath;

  /// Reports a newly selected option path; null renders the control disabled.
  final ValueChanged<String>? onSelect;

  /// Above this number of options the segmented control is replaced by radios.
  static const int _segmentedMaxOptions = 3;

  /// Longest option label that still fits one segment (inclusive).
  static const int _segmentedMaxLabelLength = 30;

  bool get _useSegmented =>
      group.options.length <= _segmentedMaxOptions &&
      group.options.every(
        (option) => _optionLabel(option).length <= _segmentedMaxLabelLength,
      );

  String _optionLabel(SheetField option) =>
      fieldDisplayName(option.path.split('.').last, option.schema);

  @override
  Widget build(BuildContext context) =>
      _useSegmented ? _segmented(context) : _radios(context);

  Widget _segmented(BuildContext context) {
    final theme = Theme.of(context);
    return SegmentedButton<String>(
      key: Key('sheet-exclusive-${group.id}'),
      segments: [
        for (final option in group.options)
          ButtonSegment<String>(
            value: option.path,
            label: Text(
              _optionLabel(option),
              key: Key('sheet-exclusive-option-${option.path}'),
              style: theme.textTheme.bodyMedium,
            ),
          ),
      ],
      selected: selectedPath == null ? const <String>{} : {selectedPath!},
      // Empty is only valid while nothing is selected; keeping it false once an
      // option is active prevents a tap from clearing the choice.
      emptySelectionAllowed: selectedPath == null,
      showSelectedIcon: false,
      style: SegmentedButton.styleFrom(
        minimumSize: const Size(0, AppSizes.controlHeight),
        textStyle: theme.textTheme.bodyMedium,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.control),
        ),
      ),
      onSelectionChanged:
          onSelect == null
              ? null
              : (selection) {
                if (selection.isEmpty) return;
                onSelect!(selection.first);
              },
    );
  }

  Widget _radios(BuildContext context) {
    final theme = Theme.of(context);
    return RadioGroup<String>(
      groupValue: selectedPath,
      onChanged:
          onSelect == null
              ? (_) {}
              : (value) {
                if (value != null) onSelect!(value);
              },
      child: Column(
        key: Key('sheet-exclusive-${group.id}'),
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final option in group.options)
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Radio<String>(
                  key: Key('sheet-exclusive-option-${option.path}'),
                  value: option.path,
                  enabled: onSelect != null,
                ),
                Text(_optionLabel(option), style: theme.textTheme.bodyMedium),
              ],
            ),
        ],
      ),
    );
  }
}
