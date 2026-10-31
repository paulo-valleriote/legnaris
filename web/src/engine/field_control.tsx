/**
 * Control rendering for one sheet field, chosen by the field's own schema
 * (ENG-07 … ENG-13): an input for editable leaves, an on/off toggle for `bool`
 * and `config.*` leaves, and a read-only value described as "Calculated" for derived
 * leaves. Controlled by construction: the value arrives as a prop and every
 * edit leaves through `onChanged`; only the numeric input's display text is
 * held locally, and it is presentational.
 */

import { useEffect, useId, useReducer, useRef, useState } from 'react';
import type { ChangeEvent, FocusEvent, KeyboardEvent, MouseEvent } from 'react';
import type { FieldSchema } from '../api/models/template';
import { message, messageWith } from '../l10n/strings';
import { fieldDisplayName, lastSegment } from './field_grouping';
import type { SheetMark } from './sheet_mark';
import { FIELD_MARKED_CLASS } from './sheet_mark';
import './engine.css';

export interface SheetFieldControlProps {
  /** Dotted path of the field, e.g. `stats.dexterity` (ENG-08, ENG-24). */
  readonly path: string;
  /** The leaf contract that decides which control is drawn (ENG-07 … ENG-11). */
  readonly schema: FieldSchema;
  /** The stored value, as held by the caller. */
  readonly value: unknown;
  /** The last preview's marking of this leaf; tints the control (ENG-22). */
  readonly mark?: SheetMark;
  /** Reports an edit; absent renders the control inert (ENG-13). */
  readonly onChanged?: (value: unknown) => void;
  /**
   * The id of the Row label that names a read-only value; given only for a lone Row, where the
   * visible label is the name. Without it (a table cell has no Row label of its own) the value is
   * named by its own `aria-label`.
   */
  readonly labelId?: string;
  /** The id of an element describing an editable number (the derived values beside it); see `NumericControl`. */
  readonly describedBy?: string;
}

/** A `config.*` proxy leaf is decided by its first path segment, the backend's 0/1 rule (ENG-08). */
function isConfigProxy(path: string): boolean {
  return path.split('.')[0] === 'config';
}

function isToggle(path: string, schema: FieldSchema): boolean {
  return schema.type === 'bool' || isConfigProxy(path);
}

/** A stored value counts as on when it is `true` or a non-zero number (ENG-08). */
function valueIsOn(value: unknown): boolean {
  if (value === true) {
    return true;
  }
  return typeof value === 'number' && value !== 0;
}

function formatFieldValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '';
  }
  return String(value);
}

/** Renders one leaf of a sheet, choosing the control from the leaf's own schema (ENG-07 … ENG-13). */
export function SheetFieldControl({
  path,
  schema,
  value,
  mark,
  onChanged,
  labelId,
  describedBy,
}: SheetFieldControlProps) {
  const label = fieldDisplayName(lastSegment(path), schema);
  const marked = mark !== undefined;
  const interactive = onChanged !== undefined;

  if (!schema.isEditable) {
    return (
      <ReadOnlyValue
        label={label}
        labelId={labelId}
        schema={schema}
        value={value}
        calculated={!marked}
        marked={marked}
      />
    );
  }

  if (isToggle(path, schema)) {
    return (
      <ToggleControl
        schema={schema}
        value={value}
        label={label}
        disabled={!interactive}
        marked={marked}
        onToggle={onChanged}
      />
    );
  }

  if (schema.type === 'number') {
    return (
      <NumericControl
        value={value}
        label={label}
        disabled={!interactive}
        marked={marked}
        steppable={labelId !== undefined}
        describedBy={describedBy}
        onChanged={onChanged}
      />
    );
  }

  if (schema.type === 'text' || schema.type === 'string') {
    return (
      <input
        type="text"
        className={`lg-field${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
        aria-label={label}
        placeholder={message('engine.notSet')}
        value={formatFieldValue(value)}
        disabled={!interactive}
        onChange={(event) => onChanged?.(event.target.value)}
      />
    );
  }

  return (
    <ReadOnlyValue
      label={label}
      labelId={labelId}
      schema={schema}
      value={value}
      calculated={false}
      marked={marked}
    />
  );
}

interface ReadOnlyValueProps {
  readonly label: string;
  readonly labelId?: string;
  readonly schema: FieldSchema;
  readonly value: unknown;
  readonly calculated: boolean;
  readonly marked: boolean;
}

/**
 * A read-only value, drawn for derived leaves and for editable leaves of an unrecognized type.
 * It is a named `group`, never a live region: a native `<output>` or `role="status"` would make
 * every derived value announce on its own change, and a Preview recalculates dozens at once. The
 * sheet footer owns the single polite announcement of that cascade. `aria-label` is only valid on
 * an element with a role, which is why the span carries `group` rather than staying generic. In a
 * lone Row the group is named by the Row's own visible label (`aria-labelledby`), so the name has
 * one source; a table cell, which has no Row label of its own, keeps `aria-label`.
 *
 * The word "Calculated" is not drawn: the recessed readout carries the derived meaning and the sheet
 * explains it once, so a table of readouts does not repeat it on every Row. The word stays as a
 * visually hidden description (`aria-describedby`), so assistive technology reads it unchanged.
 */
function ReadOnlyValue({ label, labelId, schema, value, calculated, marked }: ReadOnlyValueProps) {
  const numeric = schema.type === 'number';
  const derived = !schema.isEditable;
  const descriptionId = useId();
  return (
    <span
      className={`lg-field__readonly${derived ? ' lg-field__readonly--derived' : ''}${
        numeric ? ' lg-field__readonly--numeric' : ''
      }${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
      role="group"
      aria-label={labelId === undefined ? label : undefined}
      aria-labelledby={labelId}
      aria-describedby={calculated ? descriptionId : undefined}
    >
      <span className="lg-field__value" title={schema.formula}>
        {formatFieldValue(value)}
      </span>
      {calculated && (
        <span className="lg-field__calculated" id={descriptionId}>
          {message('engine.calculated')}
        </span>
      )}
    </span>
  );
}

interface ToggleControlProps {
  readonly schema: FieldSchema;
  readonly value: unknown;
  readonly label: string;
  readonly disabled: boolean;
  readonly marked: boolean;
  readonly onToggle?: (value: unknown) => void;
}

function ToggleControl({ schema, value, label, disabled, marked, onToggle }: ToggleControlProps) {
  const emitsBoolean = schema.type === 'bool';
  return (
    <label className={`lg-field--toggle${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}>
      <input
        type="checkbox"
        className="lg-field__toggle"
        aria-label={label}
        checked={valueIsOn(value)}
        disabled={disabled}
        onChange={(event) =>
          onToggle?.(emitsBoolean ? event.target.checked : event.target.checked ? 1 : 0)
        }
      />
    </label>
  );
}

interface NumericControlProps {
  readonly value: unknown;
  readonly label: string;
  readonly disabled: boolean;
  readonly marked: boolean;
  /** Whether the focused field offers the minus and plus steppers; see `NumericControl`. */
  readonly steppable: boolean;
  readonly describedBy?: string;
  readonly onChanged?: (value: unknown) => void;
}

/** What the text of a numeric input stands for, decided by `resolveNumericText`. */
type NumericText =
  | { readonly kind: 'empty' }
  | { readonly kind: 'pending' }
  | { readonly kind: 'invalid' }
  | { readonly kind: 'number'; readonly value: number };

const PENDING_NUMERIC_TEXT = /^-?\.?$/;

/**
 * Classifies the typed text of a numeric input. A sign or a point typed so far (`-`, `.`, `-.`) is
 * `pending`: displayable, but not yet a value. Text that never becomes a finite number (`abc`,
 * `--3`, `3-`, `1e999`) is `invalid`. A negative zero resolves to `0`, so `-0` never reaches the
 * draft as a distinct value that JSON would silently flatten.
 */
function resolveNumericText(raw: string): NumericText {
  const trimmed = raw.trim();
  if (trimmed === '') {
    return { kind: 'empty' };
  }
  if (PENDING_NUMERIC_TEXT.test(trimmed)) {
    return { kind: 'pending' };
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return { kind: 'invalid' };
  }
  return { kind: 'number', value: parsed === 0 ? 0 : parsed };
}

/** Flips the leading minus of a numeric text: `5` becomes `-5`, `-5` becomes `5`, empty becomes `-`. */
function toggleNumericSign(text: string): string {
  const trimmed = text.trim();
  return trimmed.startsWith('-') ? trimmed.slice(1) : `-${trimmed}`;
}

/** The plus-over-minus mark of the sign toggle, drawn on `currentColor`. */
function SignToggleIcon() {
  return (
    <svg className="lg-field__icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M12 3.5v8M8 7.5h8M8 18h8" strokeLinecap="round" />
    </svg>
  );
}

/** The minus or plus mark of a stepper, drawn on `currentColor`. */
function StepIcon({ direction }: { readonly direction: StepDirection }) {
  return (
    <svg className="lg-field__icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d={direction === 1 ? 'M12 6v12M6 12h12' : 'M6 12h12'} strokeLinecap="round" />
    </svg>
  );
}

type StepDirection = 1 | -1;

const NUMERIC_STEP = 1;

function decimalPlaces(value: number): number {
  const text = String(value);
  if (text.includes('e')) {
    return 0;
  }
  const point = text.indexOf('.');
  return point === -1 ? 0 : text.length - point - 1;
}

/**
 * The value one step away from the numeric input's text, or `null` when no step is possible. A
 * text that holds no number yet (empty, or a pending `-`, `.`, `-.`) steps from zero, so `-` then
 * plus gives `1` and `-` then minus gives `-1`. The sum is rounded to the decimals of the value it
 * started from, so `0.1 + 0.2`-style drift never reaches the draft, and a result that is not
 * finite or that a number this large cannot tell from its start is refused instead of reported.
 */
function stepNumericText(text: string, direction: StepDirection): number | null {
  const resolved = resolveNumericText(text);
  if (resolved.kind === 'invalid') {
    return null;
  }
  const base = resolved.kind === 'number' ? resolved.value : 0;
  const raw = base + direction * NUMERIC_STEP;
  const rounded = Number(raw.toFixed(Math.min(decimalPlaces(base), 20)));
  if (!Number.isFinite(rounded) || rounded === base) {
    return null;
  }
  return rounded === 0 ? 0 : rounded;
}

/**
 * A text input with a finite-number guard (ENG-12): the display text is local so a decimal or a
 * negative number can be typed in steps. An empty field reports a cleared value; a sign or point
 * typed so far (`-`, `.`, `-.`) stays on screen without reporting anything; a text that can never
 * resolve to a finite number leaves the stored value and the shown text unchanged. Leaving the
 * field with only a pending text restores the last reported value's text.
 *
 * Editing on a phone is the same field made faster. Focus selects the whole text, so the next
 * keystroke replaces the value; a tap on an already focused field places the caret as usual, and a
 * disabled field selects nothing. While the field has focus, a sign toggle stands in for the minus
 * key that the iOS decimal pad lacks, and, in a lone Row (`steppable`), minus and plus steppers
 * add one tap per step. The buttons keep the input focused (not tab stops, `mousedown` prevented)
 * and report through the same path as typing; ArrowDown and ArrowUp are the keyboard way to step.
 * A table cell is too narrow for the steppers (a cell is a fraction of a phone-width row, a stepper
 * is a full control wide), so it keeps the sign toggle only. The field schema carries no step,
 * minimum or maximum, so a step is always one.
 */
function NumericControl({
  value,
  label,
  disabled,
  marked,
  steppable,
  describedBy,
  onChanged,
}: NumericControlProps) {
  const inputId = useId();
  const [text, setText] = useState(() => formatFieldValue(value));
  const lastEmitted = useRef<unknown>(value);
  const inputRef = useRef<HTMLInputElement>(null);
  const [, forceRender] = useReducer((tick: number) => tick + 1, 0);

  useEffect(() => {
    if (!Object.is(value, lastEmitted.current)) {
      lastEmitted.current = value;
      setText(formatFieldValue(value));
    }
  }, [value]);

  const applyText = (raw: string) => {
    const resolved = resolveNumericText(raw);
    if (resolved.kind === 'invalid') {
      forceRender();
      return;
    }
    setText(raw);
    if (resolved.kind === 'pending') {
      return;
    }
    const next = resolved.kind === 'empty' ? null : resolved.value;
    lastEmitted.current = next;
    onChanged?.(next);
  };

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    applyText(event.target.value);
  };

  const handleBlur = () => {
    if (resolveNumericText(text).kind === 'pending') {
      setText(formatFieldValue(lastEmitted.current));
    }
  };

  const handleFocus = (event: FocusEvent<HTMLInputElement>) => {
    const input = event.currentTarget;
    const textAtFocus = input.value;
    requestAnimationFrame(() => {
      if (
        input.disabled ||
        input.ownerDocument.activeElement !== input ||
        input.value !== textAtFocus
      ) {
        return;
      }
      input.select();
      input.setSelectionRange(0, input.value.length);
    });
  };

  const handleToggleSign = () => {
    applyText(toggleNumericSign(text));
    inputRef.current?.focus();
  };

  const step = (direction: StepDirection) => {
    const next = stepNumericText(text, direction);
    if (next !== null) {
      applyText(String(next));
    }
    inputRef.current?.focus();
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') {
      return;
    }
    event.preventDefault();
    step(event.key === 'ArrowUp' ? 1 : -1);
  };

  const keepFocus = (event: MouseEvent<HTMLButtonElement>) => event.preventDefault();

  return (
    <div className="lg-field__slot">
      <div
        className={`lg-field--numeric-wrapper${marked ? ` ${FIELD_MARKED_CLASS}` : ''}${
          !disabled && steppable ? ' lg-field--steppable' : ''
        }`}
      >
        {!disabled && (
          <button
            type="button"
            className="lg-field__sign"
            tabIndex={-1}
            aria-label={messageWith('engine.toggleSign', { label })}
            onMouseDown={keepFocus}
            onClick={handleToggleSign}
          >
            <SignToggleIcon />
          </button>
        )}
        <input
          ref={inputRef}
          id={inputId}
          type="text"
          inputMode="decimal"
          className={`lg-field lg-field--numeric${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
          aria-label={label}
          aria-describedby={describedBy}
          value={text}
          disabled={disabled}
          onChange={handleChange}
          onBlur={handleBlur}
          onFocus={handleFocus}
          onKeyDown={disabled ? undefined : handleKeyDown}
        />
        {!disabled && steppable && (
          <>
            <button
              type="button"
              className="lg-field__step"
              tabIndex={-1}
              aria-label={messageWith('engine.decrease', { label })}
              onMouseDown={keepFocus}
              onClick={() => step(-1)}
            >
              <StepIcon direction={-1} />
            </button>
            <button
              type="button"
              className="lg-field__step"
              tabIndex={-1}
              aria-label={messageWith('engine.increase', { label })}
              onMouseDown={keepFocus}
              onClick={() => step(1)}
            >
              <StepIcon direction={1} />
            </button>
          </>
        )}
      </div>
    </div>
  );
}
