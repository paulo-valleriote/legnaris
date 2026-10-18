export interface FieldProps {
  label: string;
  error?: string;
  disabled?: boolean;
  numeric?: boolean;
  value?: string;
  placeholder?: string;
  onChange?: (v: string) => void;
  name?: string;
}

export function Field({
  label,
  error,
  disabled,
  numeric,
  value = '',
  placeholder,
  onChange,
  name,
}: FieldProps) {
  const id = name || label.toLowerCase().replace(/\s+/g, '-');
  return (
    <label className="lg-field" htmlFor={id}>
      <span className="lg-field__label">{label}</span>
      <input
        id={id}
        name={name || id}
        type={numeric ? 'text' : 'text'}
        className={`lg-field__input ${numeric ? 'lg-field--numeric' : ''} ${error ? 'lg-field--invalid' : ''}`}
        value={value}
        disabled={disabled}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : undefined}
        placeholder={placeholder}
        onChange={(e) => onChange?.(e.target.value)}
        style={{ textAlign: numeric ? 'right' : 'left', fontVariantNumeric: 'tabular-nums' }}
      />
      {error ? (
        <span id={`${id}-error`} className="lg-field__error">
          {error}
        </span>
      ) : null}
    </label>
  );
}
