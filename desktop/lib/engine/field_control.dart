/// Control rendering for one sheet field, chosen by the field's schema rule
/// (RND-06 … RND-13): an input for editable fields, a toggle for `config.*`
/// and bool fields, a read-only value with a "Calculated" highlight for
/// derived fields. Controlled by construction — the value comes in through
/// [SheetFieldControl.value] and edits go out through
/// [SheetFieldControl.onChanged]; the widget owns no domain state.
library;

import 'package:flutter/material.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// One field control of the schema-driven sheet (Tela 2).
class SheetFieldControl extends StatefulWidget {
  const SheetFieldControl({
    super.key,
    required this.path,
    required this.schema,
    required this.value,
    this.onChanged,
  });

  /// Dotted path of the field, e.g. `stats.dexterity`.
  final String path;

  /// Schema that decides which control is drawn.
  final FieldSchema schema;

  /// Current value of the field, as held by the caller.
  final Object? value;

  /// Reports an edit for this field; null keeps the control non-interactive.
  final ValueChanged<Object?>? onChanged;

  @override
  State<SheetFieldControl> createState() => _SheetFieldControlState();
}

class _SheetFieldControlState extends State<SheetFieldControl> {
  late final TextEditingController _controller;
  Object? _lastEmitted;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: _formatValue(widget.value));
  }

  @override
  void didUpdateWidget(covariant SheetFieldControl oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Resync only a genuinely external change: the controlled host echoes
    // back what this widget emitted, and resetting on that echo would move
    // the caret while the user types.
    if (widget.value != oldWidget.value && widget.value != _lastEmitted) {
      _controller.text = _formatValue(widget.value);
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// `config.*` is the backend's 0/1 proxy node (contract rule, not a
  /// per-system literal): its first path segment alone makes it a toggle.
  bool get _isConfigProxy => widget.path.split('.').first == 'config';

  bool get _isToggle => widget.schema.type == 'bool' || _isConfigProxy;

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
        enabled: widget.onChanged != null,
        keyboardType:
            numeric
                ? const TextInputType.numberWithOptions(decimal: true)
                : TextInputType.text,
        style: Theme.of(context).textTheme.bodyMedium,
        decoration: const InputDecoration(
          isDense: true,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.all(
              Radius.circular(AppRadius.control),
            ),
          ),
          contentPadding: EdgeInsets.symmetric(
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
    return SizedBox(
      width: AppSizes.fieldControlWidth,
      child: Align(
        alignment: Alignment.centerLeft,
        child: Switch(
          key: Key('sheet-toggle-${widget.path}'),
          value: _isOn(widget.value),
          onChanged:
              widget.onChanged == null
                  ? null
                  : (next) {
                    final Object emitted = emitsBool ? next : (next ? 1 : 0);
                    _lastEmitted = emitted;
                    widget.onChanged!(emitted);
                  },
        ),
      ),
    );
  }

  Widget _readOnly(BuildContext context, {required bool calculated}) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final value = Text(
      _formatValue(widget.value),
      key: Key('sheet-value-${widget.path}'),
      style: theme.textTheme.bodyMedium,
    );
    final formula = widget.schema.formula;
    return SizedBox(
      width: AppSizes.fieldControlWidth,
      child: Row(
        children: [
          Expanded(
            child: Align(
              alignment: Alignment.centerLeft,
              child:
                  formula == null
                      ? value
                      : Tooltip(message: formula, child: value),
            ),
          ),
          if (calculated) ...[
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
    final Object? emitted = numeric ? num.tryParse(text.trim()) : text;
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
    if (value is double && value.isFinite && value == value.truncateToDouble()) {
      return value.toInt().toString();
    }
    return value.toString();
  }
}
