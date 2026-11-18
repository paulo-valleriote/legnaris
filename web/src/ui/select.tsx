/* eslint-disable jsx-a11y/no-autofocus */
import { useId, type ReactNode, type Ref } from 'react';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  label: string;
  id?: string;
  name?: string;
  value?: string;
  options?: readonly SelectOption[];
  children?: ReactNode;
  error?: string;
  hint?: string;
  describedBy?: string;
  disabled?: boolean;
  required?: boolean;
  autoFocus?: boolean;
  selectRef?: Ref<HTMLSelectElement>;
  onChange?: (v: string) => void;
  onBlur?: () => void;
}

function slug(label: string): string {
  return label.toLowerCase().replace(/\s+/g, '-');
}

export function Select({
  label,
  id,
  name,
  value = '',
  options,
  children,
  error,
  hint,
  describedBy: extraDescribedBy,
  disabled,
  required,
  autoFocus,
  selectRef,
  onChange,
  onBlur,
}: SelectProps) {
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

  return (
    <div className="lg-form-field">
      <label className="lg-form-field__label" htmlFor={fieldId}>
        {label}
      </label>
      <div className="lg-form-field__control">
        <select
          ref={selectRef}
          id={fieldId}
          name={name ?? fieldId}
          className={`lg-form-field__input lg-form-field__select${error === undefined ? '' : ' lg-form-field__input--invalid'}`}
          value={value}
          disabled={disabled}
          required={required}
          autoFocus={autoFocus}
          aria-invalid={error === undefined ? false : true}
          aria-describedby={describedBy}
          onChange={(event) => onChange?.(event.target.value)}
          onBlur={onBlur}
        >
          {children ??
            options?.map((option) => (
              <option key={option.value} value={option.value} disabled={option.disabled}>
                {option.label}
              </option>
            ))}
        </select>
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
