import { useEffect, useRef } from 'react';
import { focusTrap } from './focus_trap';

export interface DialogProps {
  open: boolean;
  title: string;
  description?: string;
  onClose: () => void;
  children?: React.ReactNode;
}

export function Dialog({ open, title, description, onClose, children }: DialogProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open || !panelRef.current) return;
    panelRef.current.focus();
    const trap = focusTrap(panelRef.current);
    const onEsc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onEsc);
    return () => { document.removeEventListener('keydown', onEsc); trap.release(); };
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="lg-dialog__scrim" role="presentation" onClick={onClose} aria-modal="true" aria-label={title}>
      <div ref={panelRef} className="lg-dialog__panel" role="dialog" aria-modal="true" tabIndex={-1} onClick={(e) => e.stopPropagation()}>
        <h2 className="lg-dialog__title">{title}</h2>
        {description ? <p className="lg-dialog__desc">{description}</p> : null}
        {children}
      </div>
    </div>
  );
}
