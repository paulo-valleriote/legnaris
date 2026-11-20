import { useId, type KeyboardEventHandler, type ReactNode, type Ref } from 'react';

export type FieldType = 'text' | 'password' | 'email' | 'number';
export type FieldInputMode =
  'none' | 'text' | 'decimal' | 'numeric' | 'tel' | 'search' | 'email' | 'url';

export interface FieldProps {
  label: string;
  id?: string;
  name?: string;
  type?: FieldType;
  value?: string;
  placeholder?: string;
  error?: string;
  hint?: string;
  describedBy?: string;
  disabled?: boolean;
  readOnly?: boolean;
  numeric?: boolean;
  required?: boolean;
  multiline?: boolean;
  rows?: number;
  autoComplete?: string;
  inputMode?: FieldInputMode;
  maxLength?: number;
  pattern?: string;
  autoFocus?: boolean;
  inputRef?: Ref<HTMLInputElement>;
  onChange?: (v: string) => void;
  onBlur?: () => void;
  onKeyDown?: KeyboardEventHandler<HTMLInputElement>;
  trailing?: ReactNode;
}

function slug(label: string): string {
  return label.toLowerCase().replace(/\s+/g, '-');
}

export function Field({
  label,
  id,
  name,
  type = 'text',
  value = '',
  placeholder,
  error,
  hint,
  describedBy: extraDescribedBy,
  disabled,
  readOnly,
  numeric,
  required,
  multiline,
  rows,
  autoComplete,
  inputMode,
  maxLength,
  pattern,
  autoFocus,
  inputRef,
  onChange,
  onBlur,
  onKeyDown,
  trailing,
}: FieldProps) {
  const generatedId = useId();
  const fieldId = id ?? name ?? (slug(label) || generatedId);
  const hintId = `${fieldId}-hint`;
  const errorId = `${fieldId}-error`;
  const describedBy =
    [
      hint === undefined ? undefined : hintId,
      extraDescribedBy,
      error === undefined ? undefined : errorId,
    ]
      .filter((value): value is string => value !== undefined && value !== '')
      .join(' ') || undefined;
  const controlClasses = `lg-form-field__input${multiline ? ' lg-form-field__textarea' : ''}${
    numeric ? ' lg-form-field__input--numeric' : ''
  }${error === undefined ? '' : ' lg-form-field__input--invalid'}`;

  const shared = {
    id: fieldId,
    name: name ?? fieldId,
    className: controlClasses,
    value,
    disabled,
    readOnly,
    required,
    autoComplete,
    inputMode,
    maxLength,
    autoFocus,
    'aria-invalid': error !== undefined,
    'aria-describedby': describedBy,
    placeholder,
    onChange: (event: { target: { value: string } }) => onChange?.(event.target.value),
    onBlur,
  };

  return (
    <div className="lg-form-field">
      <label className="lg-form-field__label" htmlFor={fieldId}>
        {label}
      </label>
      <div className="lg-form-field__control">
        {multiline === true ? (
          <textarea {...shared} rows={rows} />
        ) : (
          <input {...shared} ref={inputRef} type={type} pattern={pattern} onKeyDown={onKeyDown} />
        )}
        {trailing === undefined ? null : (
          <span className="lg-form-field__trailing">{trailing}</span>
        )}
      </div>
      {hint === undefined ? null : (
        <span id={hintId} className="lg-form-field__hint">
          {hint}
        </span>
      )}
      {error === undefined ? null : (
        <span id={errorId} className="lg-form-field__error" aria-live="polite">
          {error}
        </span>
      )}
    </div>
  );
}
