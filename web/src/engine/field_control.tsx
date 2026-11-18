import { useEffect, useId, useReducer, useRef, useState } from 'react';
import type { ChangeEvent, FocusEvent, KeyboardEvent, MouseEvent } from 'react';
import type { FieldSchema } from '../api/models/template';
import { message, messageWith } from '../l10n/strings';
import { MinusIcon, PlusIcon } from '../ui/icons';
import { fieldDisplayName, lastSegment } from './field_grouping';
import type { SheetMark } from './sheet_mark';
import { FIELD_MARKED_CLASS } from './sheet_mark';
import './engine.css';

export interface SheetFieldControlProps {
  readonly path: string;
  readonly schema: FieldSchema;
  readonly value: unknown;
  readonly mark?: SheetMark;
  readonly onChanged?: (value: unknown) => void;
  readonly labelId?: string;
  readonly describedBy?: string;
}

function isConfigProxy(path: string): boolean {
  return path.split('.')[0] === 'config';
}

function isToggle(path: string, schema: FieldSchema): boolean {
  return schema.type === 'bool' || isConfigProxy(path);
}

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

function ReadOnlyValue({ label, labelId, schema, value, calculated, marked }: ReadOnlyValueProps) {
  const numeric = schema.type === 'number';
  const derived = !schema.isEditable;
  const descriptionId = useId();
  const valueId = useId();
  const shown = formatFieldValue(value);
  const formula =
    derived && schema.formula !== undefined && schema.formula !== '' ? schema.formula : undefined;
  return (
    <span
      className={`lg-field__readonly${derived ? ' lg-field__readonly--derived' : ''}${
        numeric ? ' lg-field__readonly--numeric' : ''
      }${marked ? ` ${FIELD_MARKED_CLASS}` : ''}`}
      role="group"
      aria-label={labelId === undefined ? (shown === '' ? label : `${label}: ${shown}`) : undefined}
      aria-labelledby={labelId === undefined ? undefined : `${labelId} ${valueId}`}
      aria-describedby={calculated ? descriptionId : undefined}
      title={formula}
    >
      {derived && (
        <span className="lg-field__fx" aria-hidden="true">
          {message('engine.derivedMark')}
        </span>
      )}
      <span className="lg-field__value" id={valueId}>
        {shown}
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
  readonly steppable: boolean;
  readonly describedBy?: string;
  readonly onChanged?: (value: unknown) => void;
}

type NumericText =
  | { readonly kind: 'empty' }
  | { readonly kind: 'pending' }
  | { readonly kind: 'invalid' }
  | { readonly kind: 'number'; readonly value: number };

const PENDING_NUMERIC_TEXT = /^-?\.?$/;

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

function toggleNumericSign(text: string): string {
  const trimmed = text.trim();
  return trimmed.startsWith('-') ? trimmed.slice(1) : `-${trimmed}`;
}

function SignToggleIcon() {
  return (
    <svg className="lg-field__icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M12 3.5v8M8 7.5h8M8 18h8" strokeLinecap="round" />
    </svg>
  );
}

function StepIcon({ direction }: { readonly direction: StepDirection }) {
  if (direction === 1) {
    return <PlusIcon className="lg-field__icon" />;
  }
  return <MinusIcon className="lg-field__icon" />;
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
