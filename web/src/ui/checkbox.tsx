export interface CheckboxProps {
  label: string;
  checked?: boolean;
  disabled?: boolean;
  invalid?: boolean;
  onChange?: (v: boolean) => void;
  name?: string;
}

export function Checkbox({ label, checked = false, disabled, invalid, onChange, name }: CheckboxProps) {
  const id = name || label.toLowerCase().replace(/\s+/g, '-');
  return (
    <label className="lg-checkbox" htmlFor={id}>
      <input
        id={id}
        name={name || id}
        type="checkbox"
        className={`lg-checkbox__box ${invalid ? 'lg-checkbox--invalid' : ''}`}
        checked={checked}
        disabled={disabled}
        aria-invalid={invalid || undefined}
        onChange={(e) => onChange?.(e.target.checked)}
      />
      <span className="lg-checkbox__label">{label}</span>
    </label>
  );
}
