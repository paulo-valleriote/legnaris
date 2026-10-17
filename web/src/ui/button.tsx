import { message } from '../l10n/strings';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'outline' | 'danger';
export type ButtonSize = 'sm' | 'md';

export interface ButtonProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  disabled?: boolean;
  loading?: boolean;
  label?: string;
  name?: string;
  onClick?: () => void;
}

export function Button({
  variant = 'primary',
  size = 'md',
  disabled = false,
  loading = false,
  label,
  name,
  onClick,
}: ButtonProps) {
  const busy = disabled || loading;
  const accessibleName = name || label || '';
  return (
    <button
      type="button"
      className={`lg-button lg-button--${variant} lg-button--${size}`}
      disabled={busy}
      aria-disabled={busy ? true : undefined}
      aria-busy={loading ? true : undefined}
      aria-label={accessibleName || undefined}
      onClick={busy ? undefined : onClick}
    >
      {loading && <span aria-hidden="true">●</span>}
      {label || accessibleName}
    </button>
  );
}
