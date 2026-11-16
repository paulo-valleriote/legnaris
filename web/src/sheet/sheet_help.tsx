import type { Ref } from 'react';
import { message } from '../l10n/strings';

export const HELP_PANEL_ID = 'lg-sheet-help';

export interface HelpToggleProps {
  readonly open: boolean;
  readonly onToggle: () => void;
  readonly buttonRef?: Ref<HTMLButtonElement>;
}

export function HelpToggle({ open, onToggle, buttonRef }: HelpToggleProps) {
  return (
    <button
      type="button"
      ref={buttonRef}
      className="lg-button lg-button--ghost lg-button--sm"
      aria-expanded={open}
      aria-controls={HELP_PANEL_ID}
      onClick={onToggle}
    >
      {message('sheet.help')}
    </button>
  );
}

export interface HelpPanelProps {
  readonly onDismiss: () => void;
}

export function HelpPanel({ onDismiss }: HelpPanelProps) {
  return (
    <div
      className="lg-sheet__help"
      id={HELP_PANEL_ID}
      role="region"
      aria-label={message('sheet.helpTitle')}
      data-testid="sheet-help"
    >
      <div className="lg-sheet__help-body">
        <p className="lg-sheet__help-text">{message('sheet.helpDerived')}</p>
        <p className="lg-sheet__help-text">{message('sheet.helpPreview')}</p>
      </div>
      <button
        type="button"
        className="lg-button lg-button--ghost lg-sheet__help-dismiss"
        aria-label={message('sheet.helpDismiss')}
        onClick={onDismiss}
      >
        <svg
          className="lg-sheet__help-close"
          viewBox="0 0 24 24"
          aria-hidden="true"
          focusable="false"
        >
          <path d="M6 6l12 12M18 6L6 18" />
        </svg>
      </button>
    </div>
  );
}
