/**
 * Control rendering for one sheet field, chosen by the field's own schema
 * (ENG-07 … ENG-13): an input for editable leaves, an on/off toggle for `bool`
 * and `config.*` leaves, and a read-only value marked "Calculated" for derived
 * leaves. Controlled by construction: the value arrives as a prop and every
 * edit leaves through `onChanged`; only the numeric input's display text is
 * held locally, and it is presentational.
 */

import { useEffect, useReducer, useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import type { FieldSchema } from '../api/models/template';
import { message } from '../l10n/strings';
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
}: SheetFieldControlProps) {
  const label = fieldDisplayName(lastSegment(path), schema);
  const marked = mark !== undefined;
  const interactive = onChanged !== undefined;

  if (!schema.isEditable) {
    return (
      <ReadOnlyValue
        label={label}
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
        value={formatFieldValue(value)}
        disabled={!interactive}
        onChange={(event) => onChanged?.(event.target.value)}
      />
    );
  }

  return (
    <ReadOnlyValue label={label} schema={schema} value={value} calculated={false} marked={marked} />
  );
}

interface ReadOnlyValueProps {
  readonly label: string;
  readonly schema: FieldSchema;
  readonly value: unknown;
  readonly calculated: boolean;
  readonly marked: boolean;
}

function ReadOnlyValue({ label, schema, value, calculated, marked }: ReadOnlyValueProps) {
  return (
    <span
      className={`lg-field__readonly${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
      role="status"
      aria-label={label}
    >
      <span className="lg-field__value" title={schema.formula}>
        {formatFieldValue(value)}
      </span>
      {calculated && <span className="lg-field__calculated">{message('engine.calculated')}</span>}
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
  readonly onChanged?: (value: unknown) => void;
}

/**
 * A text input with a finite-number guard (ENG-12): the display text is local so
 * a decimal can be typed, an empty field reports a cleared value, and a text the
 * platform does not resolve to a finite number leaves the stored value unchanged.
 */
function NumericControl({ value, label, disabled, marked, onChanged }: NumericControlProps) {
  const [text, setText] = useState(() => formatFieldValue(value));
  const lastEmitted = useRef<unknown>(value);
  const [, forceRender] = useReducer((tick: number) => tick + 1, 0);

  useEffect(() => {
    if (!Object.is(value, lastEmitted.current)) {
      lastEmitted.current = value;
      setText(formatFieldValue(value));
    }
  }, [value]);

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const raw = event.target.value;
    const trimmed = raw.trim();
    if (trimmed === '') {
      setText('');
      lastEmitted.current = null;
      onChanged?.(null);
      return;
    }
    const parsed = Number(trimmed);
    if (!Number.isFinite(parsed)) {
      // Reject the whole edit and re-render the controlled input back to the
      // last accepted text, so a non-finite number never enters the draft.
      forceRender();
      return;
    }
    setText(raw);
    lastEmitted.current = parsed;
    onChanged?.(parsed);
  };

  return (
    <label
      htmlFor={`lg-field-num-${label}`}
      className={`lg-field--numeric-wrapper${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
    >
      <input
        id={`lg-field-num-${label}`}
        type="text"
        inputMode="decimal"
        className={`lg-field lg-field--numeric${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
        aria-label={label}
        value={text}
        disabled={disabled}
        onChange={handleChange}
      />
    </label>
  );
}
