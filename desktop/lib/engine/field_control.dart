/// Control rendering for one sheet field, chosen by the field's schema rule
/// (RND-06 … RND-13): an input for editable fields, a toggle for `config.*`
/// and bool fields, a read-only value with a "Calculated" highlight for
/// derived fields. An optional [SheetFieldControl.mark] tints the control and,
/// on a derived field, replaces that base label with the last preview's state.
/// Controlled by construction — the value comes in through
/// [SheetFieldControl.value] and edits go out through
/// [SheetFieldControl.onChanged]; the widget owns no domain state.
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/sheet_mark.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// One field control of the schema-driven sheet (Tela 2).
class SheetFieldControl extends StatefulWidget {
  const SheetFieldControl({
    super.key,
    required this.path,
    required this.schema,
    required this.value,
    this.mark,
    this.onChanged,
  });

  /// Dotted path of the field, e.g. `stats.dexterity`.
  final String path;

  /// Schema that decides which control is drawn.
  final FieldSchema schema;

  /// Current value of the field, as held by the caller.
  final Object? value;

  /// Preview marking of the last dry-run for this field, or null. Drives the
  /// change tint and, on a derived field, replaces the base "Calculated"
  /// label with the preview state.
  final SheetMark? mark;

  /// Reports an edit for this field; null keeps the control non-interactive.
  final ValueChanged<Object?>? onChanged;

  @override
  State<SheetFieldControl> createState() => _SheetFieldControlState();
}

class _SheetFieldControlState extends State<SheetFieldControl> {
  late final TextEditingController _controller;
  late final FocusNode _focusNode;
  Object? _lastEmitted;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: _formatValue(widget.value));
    _focusNode = FocusNode()..addListener(_restoreStoredValueOnBlur);
  }

  @override
  void didUpdateWidget(covariant SheetFieldControl oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Resync only a genuinely external change: the controlled host echoes
    // back what this widget emitted, and resetting on that echo would move
    // the caret while the user types. The marker must track what the display
    // now shows, or a later external change equal to a stale marker would be
    // skipped and the text would silently diverge from the stored value.
    if (widget.value != oldWidget.value && widget.value != _lastEmitted) {
      _controller.text = _formatValue(widget.value);
      _lastEmitted = widget.value;
    }
  }

  @override
  void dispose() {
    _focusNode.removeListener(_restoreStoredValueOnBlur);
    _focusNode.dispose();
    _controller.dispose();
    super.dispose();
  }

  /// Restores the display of the stored value when the user leaves the field
  /// on text the draft does not hold (a half-typed `-` or `.`, or input that
  /// failed to parse), so the control never at rest shows text that differs
  /// from what would be saved.
  void _restoreStoredValueOnBlur() {
    if (_focusNode.hasFocus || !mounted || !_isNumericInput) return;
    final text = _controller.text;
    if (text.isEmpty || _parseNumber(text) != null) return;
    _controller.text = _formatValue(widget.value);
    _lastEmitted = widget.value;
  }

  /// `config.*` is the backend's 0/1 proxy node (contract rule, not a
  /// per-system literal): its first path segment alone makes it a toggle.
  bool get _isConfigProxy => widget.path.split('.').first == 'config';

  bool get _isToggle => widget.schema.type == 'bool' || _isConfigProxy;

  /// True when this control draws a numeric input (see [build]'s branches).
  bool get _isNumericInput =>
      widget.schema.isEditable &&
      !_isToggle &&
      widget.schema.type == 'number';

  @override
  Widget build(BuildContext context) {
    if (!widget.schema.isEditable) return _readOnly(context, calculated: true);
    if (_isToggle) return _toggle();
    if (widget.schema.type == 'number') {
      return _textInput(context, numeric: true);
    }
    if (widget.schema.type == 'text' || widget.schema.type == 'string') {
      return _textInput(context, numeric: false);
    }
    return _readOnly(context, calculated: false);
  }

  Widget _textInput(BuildContext context, {required bool numeric}) {
    return SizedBox(
      width: AppSizes.fieldControlWidth,
      child: TextField(
        key: Key('sheet-input-${widget.path}'),
        controller: _controller,
        focusNode: _focusNode,
        enabled: widget.onChanged != null,
        keyboardType:
            numeric
                ? const TextInputType.numberWithOptions(decimal: true)
                : TextInputType.text,
        // `keyboardType` only shapes the IME — a desktop keyboard types
        // anything, so the numeric field guards its whole value instead.
        inputFormatters:
            numeric ? const [_NumericTextFormatter()] : null,
        style: Theme.of(context).textTheme.bodyMedium,
        decoration: InputDecoration(
          isDense: true,
          filled: widget.mark != null,
          fillColor: widget.mark == null
              ? null
              : sheetMarkTint(Theme.of(context).colorScheme),
          border: const OutlineInputBorder(
            borderRadius: BorderRadius.all(
              Radius.circular(AppRadius.control),
            ),
          ),
          contentPadding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.sm,
            vertical: AppSpacing.sm,
          ),
        ),
        onChanged: (text) => _emitText(text, numeric: numeric),
      ),
    );
  }

  Widget _toggle() {
    final bool emitsBool = widget.schema.type == 'bool';
    final tint = widget.mark == null
        ? null
        : sheetMarkTint(Theme.of(context).colorScheme);
    return SizedBox(
      width: AppSizes.fieldControlWidth,
      child: Align(
        alignment: Alignment.centerLeft,
        child: Container(
          key: Key('sheet-mark-tint-${widget.path}'),
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xs),
          decoration: tint == null
              ? null
              : BoxDecoration(
                  color: tint,
                  borderRadius: BorderRadius.circular(AppRadius.control),
                ),
          child: Checkbox(
            key: Key('sheet-checkbox-${widget.path}'),
            value: _isOn(widget.value),
            visualDensity: VisualDensity.compact,
            materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
            onChanged:
                widget.onChanged == null
                    ? null
                    : (next) {
                      final bool on = next ?? false;
                      final Object emitted = emitsBool ? on : (on ? 1 : 0);
                      _lastEmitted = emitted;
                      widget.onChanged!(emitted);
                    },
          ),
        ),
      ),
    );
  }

  Widget _readOnly(BuildContext context, {required bool calculated}) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final tint = widget.mark == null
        ? null
        : sheetMarkTint(theme.colorScheme);
    final value = Text(
      _formatValue(widget.value),
      key: Key('sheet-value-${widget.path}'),
      style: theme.textTheme.bodyMedium,
    );
    final formula = widget.schema.formula;
    final displayed = formula == null
        ? value
        : Tooltip(message: formula, child: value);
    return SizedBox(
      width: AppSizes.fieldControlWidth,
      child: Row(
        children: [
          Expanded(
            child: Align(
              alignment: Alignment.centerLeft,
              child: tint == null
                  ? displayed
                  : Container(
                      key: Key('sheet-mark-tint-${widget.path}'),
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppSpacing.xs,
                        vertical: AppSpacing.xs,
                      ),
                      decoration: BoxDecoration(
                        color: tint,
                        borderRadius: BorderRadius.circular(
                          AppRadius.control,
                        ),
                      ),
                      child: displayed,
                    ),
            ),
          ),
          // A preview-marked derived field names its state on the row label,
          // so the base category label would be redundant (RND-10 otherwise).
          if (calculated && widget.mark == null) ...[
            const SizedBox(width: AppSpacing.sm),
            Text(
              localizations.fieldCalculated,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }

  void _emitText(String text, {required bool numeric}) {
    if (!numeric) {
      _lastEmitted = text;
      widget.onChanged?.call(text);
      return;
    }
    final trimmed = text.trim();
    if (trimmed.isEmpty) {
      _lastEmitted = null;
      widget.onChanged?.call(null);
      return;
    }
    // A sign or decimal point the user is still typing (`-`, `.`) is not a
    // value yet: keep the stored one instead of reporting null for an
    // in-progress edit — the display is restored when focus leaves.
    if (_typingIntermediate.hasMatch(trimmed)) return;
    final emitted = _parseNumber(trimmed);
    _lastEmitted = emitted;
    widget.onChanged?.call(emitted);
  }

  bool _isOn(Object? value) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    return false;
  }

  String _formatValue(Object? value) {
    if (value == null) return '';
    // Integer rendering only while the double holds the integer exactly:
    // `toInt()` clamps above int64, which would display a different number
    // than the one stored (e.g. 1e300 → 9223372036854775807).
    if (value is double &&
        value.isFinite &&
        value.abs() <= _maxExactInteger &&
        value == value.truncateToDouble()) {
      return value.toInt().toString();
    }
    return value.toString();
  }
}

/// A complete decimal number: optional sign, digits with one optional
/// fraction, or a bare fraction (`.5`).
final RegExp _numericShape = RegExp(r'^[+-]?(\d+(\.\d*)?|\.\d+)$');

/// A sign or decimal point still being typed (`-`, `+.`, …) — valid text,
/// not yet a value.
final RegExp _typingIntermediate = RegExp(r'^[+-]?\.?$');

/// Largest integer a double holds exactly; beyond it integer rendering
/// would lose precision (or clamp, see `_formatValue`).
const double _maxExactInteger = 9007199254740991;

/// Parses [text] as the number a sheet field may hold.
///
/// The shape check rejects forms `num.tryParse` would otherwise accept —
/// `0x10` (hex → 16), `NaN`, `Infinity`, `1e999` (exponent) — and the
/// finiteness check catches digit runs that overflow to `Infinity`, so no
/// non-finite value can ever enter the draft.
num? _parseNumber(String text) {
  if (!_numericShape.hasMatch(text)) return null;
  final parsed = num.tryParse(text);
  if (parsed == null || (parsed is double && !parsed.isFinite)) return null;
  return parsed;
}

/// Rejects edits that would leave the numeric field showing text it cannot
/// represent; the edit reverts to the previous value instead. Whole-value
/// check, so an invalid keystroke never reaches `_emitText`.
class _NumericTextFormatter extends TextInputFormatter {
  const _NumericTextFormatter();

  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    final text = newValue.text;
    if (text.isEmpty ||
        _typingIntermediate.hasMatch(text) ||
        _numericShape.hasMatch(text)) {
      return newValue;
    }
    return oldValue;
  }
}
