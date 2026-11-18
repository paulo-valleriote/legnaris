import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { message } from '../l10n/strings';
import { BackIcon } from './icons';

export interface PageHeaderProps {
  title: string;
  titleId?: string;
  backTo?: string;
  backLabel?: string;
  meta?: ReactNode;
  actions?: ReactNode;
  id?: string;
  className?: string;
}

export function PageHeader({
  title,
  titleId,
  backTo,
  backLabel,
  meta,
  actions,
  id,
  className,
}: PageHeaderProps) {
  const back = backLabel ?? message('common.back');
  const classes =
    className === undefined || className === '' ? 'lg-page-header' : `lg-page-header ${className}`;
  return (
    <header className={classes} id={id}>
      <div className="lg-page-header__main">
        {backTo === undefined ? null : (
          <Link className="lg-page-header__back" to={backTo} aria-label={back} title={back}>
            <BackIcon />
          </Link>
        )}
        <div className="lg-page-header__titles">
          <h1 className="lg-page-header__title" id={titleId}>
            {title}
          </h1>
          {meta === undefined ? null : <div className="lg-page-header__meta">{meta}</div>}
        </div>
      </div>
      {actions === undefined ? null : <div className="lg-page-header__actions">{actions}</div>}
    </header>
  );
}
