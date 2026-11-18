import { forwardRef, useId, type MouseEvent, type ReactNode, type Ref } from 'react';
import { Link } from 'react-router-dom';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'outline' | 'danger' | 'row';
export type ButtonSize = 'sm' | 'md';
export type ButtonAriaHasPopup = boolean | 'menu' | 'listbox' | 'tree' | 'grid' | 'dialog';

export interface ButtonProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  type?: 'button' | 'submit';
  disabled?: boolean;
  loading?: boolean;
  label?: string;
  name?: string;
  children?: ReactNode;
  icon?: ReactNode;
  trailingIcon?: ReactNode;
  className?: string;
  fullWidth?: boolean;
  ariaDescribedBy?: string;
  ariaExpanded?: boolean;
  ariaControls?: string;
  ariaPressed?: boolean;
  ariaHasPopup?: ButtonAriaHasPopup;
  disabledReason?: string;
  to?: string;
  href?: string;
  id?: string;
  title?: string;
  onClick?: (event: MouseEvent<HTMLElement>) => void;
}

type ButtonElement = HTMLButtonElement | HTMLAnchorElement;

export const Button = forwardRef<ButtonElement, ButtonProps>(function Button(
  {
    variant = 'primary',
    size = 'md',
    type = 'button',
    disabled = false,
    loading = false,
    label,
    name,
    children,
    icon,
    trailingIcon,
    className,
    fullWidth = false,
    ariaDescribedBy,
    ariaExpanded,
    ariaControls,
    ariaPressed,
    ariaHasPopup,
    disabledReason,
    to,
    href,
    id,
    title,
    onClick,
  },
  ref,
) {
  const reasonId = useId();
  const isDisabled = disabled || disabledReason !== undefined;
  const blocked = loading || (isDisabled && disabledReason === undefined);
  const busy = isDisabled || loading;
  const accessibleName = name || label || '';
  const describedBy =
    [ariaDescribedBy, disabledReason === undefined ? undefined : reasonId]
      .filter((value): value is string => value !== undefined && value !== '')
      .join(' ') || undefined;
  const classes = [
    'lg-button',
    `lg-button--${variant}`,
    `lg-button--${size}`,
    fullWidth ? 'lg-button--full' : undefined,
    className,
  ]
    .filter((value): value is string => value !== undefined && value !== '')
    .join(' ');
  const aria = {
    'aria-label': accessibleName || undefined,
    'aria-expanded': ariaExpanded,
    'aria-controls': ariaControls,
    'aria-pressed': ariaPressed,
    'aria-haspopup': ariaHasPopup,
    id,
    title,
  };

  const linkMode = to !== undefined || href !== undefined;
  const iconClass = 'lg-button__icon';
  const trailingIconClass = 'lg-button__icon lg-button__icon--trailing';
  const reasonClass = 'lg-button__reason';
  const content = (
    <>
      {loading && !linkMode ? (
        <span className="lg-spinner lg-button__spinner" aria-hidden="true" />
      ) : null}
      {icon === undefined ? null : (
        <span className={iconClass} aria-hidden="true">
          {icon}
        </span>
      )}
      {children === undefined ? label || accessibleName : children}
      {trailingIcon === undefined ? null : (
        <span className={trailingIconClass} aria-hidden="true">
          {trailingIcon}
        </span>
      )}
    </>
  );

  if (to !== undefined) {
    return (
      <Link
        ref={ref as Ref<HTMLAnchorElement>}
        to={to}
        className={classes}
        {...aria}
        onClick={onClick}
      >
        {content}
      </Link>
    );
  }

  if (href !== undefined) {
    return (
      <a
        ref={ref as Ref<HTMLAnchorElement>}
        href={href}
        className={classes}
        {...aria}
        onClick={onClick}
      >
        {content}
      </a>
    );
  }

  const button = (
    <button
      ref={ref as Ref<HTMLButtonElement>}
      type={type}
      className={classes}
      disabled={blocked}
      aria-disabled={busy ? true : undefined}
      aria-busy={loading ? true : undefined}
      aria-describedby={describedBy}
      {...aria}
      onClick={busy ? undefined : onClick}
    >
      {content}
    </button>
  );

  if (disabledReason === undefined) {
    return button;
  }

  const wrapClass = `lg-button__wrap${fullWidth ? ' lg-button__wrap--full' : ''}`;

  return (
    <span className={wrapClass}>
      {button}
      <span className={reasonClass} id={reasonId}>
        {disabledReason}
      </span>
    </span>
  );
});
