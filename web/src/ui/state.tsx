import type { ReactNode } from 'react';
import { message } from '../l10n/strings';
import { Button } from './button';

function classes(base: string, className: string | undefined): string {
  return className === undefined || className === '' ? base : `${base} ${className}`;
}

export interface EmptyStateProps {
  title?: string;
  message: string;
  action?: ReactNode;
  id?: string;
  className?: string;
}

export function EmptyState({ title, message: text, action, id, className }: EmptyStateProps) {
  return (
    <div className={classes('lg-state lg-state--empty', className)} id={id}>
      {title === undefined ? null : <p className="lg-state__title">{title}</p>}
      <p className="lg-state__text">{text}</p>
      {action === undefined ? null : <div className="lg-state__action">{action}</div>}
    </div>
  );
}

export interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
  retryLabel?: string;
  id?: string;
  className?: string;
}

export function ErrorState({ message: text, onRetry, retryLabel, id, className }: ErrorStateProps) {
  return (
    <div className={classes('lg-state lg-state--error', className)} id={id} role="alert">
      <p className="lg-state__text lg-state__text--danger">{text}</p>
      {onRetry === undefined ? null : (
        <Button
          variant="secondary"
          size="md"
          label={retryLabel ?? message('common.retry')}
          onClick={onRetry}
        />
      )}
    </div>
  );
}

export interface LoadingStateProps {
  label?: string;
  id?: string;
  className?: string;
}

export function LoadingState({ label, id, className }: LoadingStateProps) {
  return (
    <div
      className={classes('lg-state lg-state--loading', className)}
      id={id}
      role="status"
      aria-busy="true"
    >
      <span className="lg-spinner" aria-hidden="true" />
      <span className="lg-state__text">{label ?? message('common.loading')}</span>
    </div>
  );
}
