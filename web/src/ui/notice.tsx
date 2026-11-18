import type { ReactNode } from 'react';

export type NoticeTone = 'info' | 'warning' | 'danger' | 'success';
export type NoticeSurface = 'app' | 'stage';

export interface NoticeProps {
  tone: NoticeTone;
  surface?: NoticeSurface;
  children: ReactNode;
  action?: ReactNode;
  live?: 'polite' | 'off';
  id?: string;
  className?: string;
}

export function Notice({
  tone,
  surface = 'app',
  children,
  action,
  live = 'off',
  id,
  className,
}: NoticeProps) {
  const classes = [
    'lg-notice',
    `lg-notice--${tone}`,
    surface === 'stage' ? 'lg-notice--stage' : undefined,
    className,
  ]
    .filter((value): value is string => value !== undefined && value !== '')
    .join(' ');

  return (
    <div
      className={classes}
      id={id}
      role={live === 'polite' ? 'status' : undefined}
      aria-live={live === 'polite' ? 'polite' : undefined}
    >
      <p className="lg-notice__text">{children}</p>
      {action === undefined ? null : <div className="lg-notice__action">{action}</div>}
    </div>
  );
}
