import { message } from '../l10n/strings';

export interface ToastProps {
  message: string;
  variant?: 'info' | 'success' | 'warning' | 'danger';
  onDismiss?: () => void;
}

export function Toast({ message: msg, variant = 'info', onDismiss }: ToastProps) {
  if (!msg || msg.trim() === '') return null;
  return (
    <aside className={`lg-toast lg-toast--${variant}`} role="status" aria-live="polite">
      <span className="lg-toast__message">{msg}</span>
      {onDismiss ? (
        <button className="lg-toast__dismiss" onClick={onDismiss} aria-label={message('ui.dismiss')}>
          ×
        </button>
      ) : null}
    </aside>
  );
}
