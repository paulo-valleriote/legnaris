import { useEffect, useId, useLayoutEffect, useRef, type ReactNode, type RefObject } from 'react';
import { focusTrap } from './focus_trap';
import { CloseIcon } from './icons';

export type DialogPlacement = 'center' | 'bottom';

export interface DialogProps {
  open: boolean;
  title: string;
  titleId?: string;
  description?: string;
  onClose: () => void;
  children?: ReactNode;
  tone?: 'default' | 'danger';
  actions?: ReactNode;
  closeLabel?: string;
  initialFocusRef?: RefObject<HTMLElement | null>;
  error?: string;
  className?: string;
  placement?: DialogPlacement;
}

const openDialogs: symbol[] = [];

function isTopDialog(token: symbol): boolean {
  return openDialogs[openDialogs.length - 1] === token;
}

function leaveStack(token: symbol): void {
  const index = openDialogs.indexOf(token);
  if (index !== -1) {
    openDialogs.splice(index, 1);
  }
}

export function Dialog({
  open,
  title,
  titleId,
  description,
  onClose,
  children,
  tone = 'default',
  actions,
  closeLabel,
  initialFocusRef,
  error,
  className,
  placement = 'center',
}: DialogProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const generatedTitleId = useId();
  const resolvedTitleId = titleId ?? generatedTitleId;
  const onCloseRef = useRef(onClose);
  const restoreRef = useRef<HTMLElement | null>(null);

  useLayoutEffect(() => {
    if (!open) return;
    restoreRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
  }, [open]);

  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    if (!open) return;
    return () => {
      const restore = restoreRef.current;
      restoreRef.current = null;
      if (restore !== null && document.contains(restore)) {
        restore.focus();
      }
    };
  }, [open]);

  useEffect(() => {
    if (!open) return undefined;
    const panel = panelRef.current;
    if (!panel) return undefined;
    const token = Symbol('dialog');
    openDialogs.push(token);
    const initial = initialFocusRef?.current ?? null;
    if (initial !== null) {
      initial.focus();
    } else if (!panel.contains(document.activeElement)) {
      panel.focus();
    }
    const trap = focusTrap(panel);
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && isTopDialog(token)) {
        onCloseRef.current();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      leaveStack(token);
      trap.release();
    };
  }, [open, initialFocusRef]);

  if (!open) return null;

  const close = () => onCloseRef.current();
  const panelClasses = [
    'lg-dialog__panel',
    tone === 'danger' ? 'lg-dialog__panel--danger' : undefined,
    className,
  ]
    .filter((value): value is string => value !== undefined && value !== '')
    .join(' ');
  const scrimClasses = [
    'lg-dialog__scrim',
    placement === 'bottom' ? 'lg-dialog__scrim--bottom' : undefined,
  ]
    .filter((value): value is string => value !== undefined)
    .join(' ');

  return (
    <div
      className={scrimClasses}
      role="presentation"
      onClick={(event) => {
        if (event.target === event.currentTarget) close();
      }}
    >
      <div
        ref={panelRef}
        className={panelClasses}
        role="dialog"
        aria-modal="true"
        aria-labelledby={resolvedTitleId}
        tabIndex={-1}
      >
        <div className="lg-dialog__header">
          <h2 className="lg-dialog__title" id={resolvedTitleId}>
            {title}
          </h2>
          {closeLabel === undefined ? null : (
            <button
              type="button"
              className="lg-dialog__close"
              aria-label={closeLabel}
              title={closeLabel}
              onClick={close}
            >
              <CloseIcon />
            </button>
          )}
        </div>
        {description ? <p className="lg-dialog__desc">{description}</p> : null}
        {children}
        {error === undefined ? null : (
          <p className="lg-dialog__error" role="alert">
            {error}
          </p>
        )}
        {actions === undefined ? null : <div className="lg-dialog__actions">{actions}</div>}
      </div>
    </div>
  );
}
