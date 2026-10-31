/**
 * The sheet's help: a small disclosure that explains the two things the page does not say by
 * itself, how a calculated number looks and what Preview does against Save. It never blocks and
 * never moves focus: {@link HelpToggle} sits among the header actions and {@link HelpPanel} opens
 * as plain text under the heading, closed again by the same control or by its close button (named `Got it`). The screen
 * opens it once by itself for a viewer who has not dismissed it and remembers the dismissal.
 */

import type { Ref } from 'react';
import { message } from '../l10n/strings';

/** The id the toggle controls and the panel carries. */
export const HELP_PANEL_ID = 'lg-sheet-help';

/** The props of {@link HelpToggle}. */
export interface HelpToggleProps {
  readonly open: boolean;
  readonly onToggle: () => void;
  readonly buttonRef?: Ref<HTMLButtonElement>;
}

/** The control that opens and closes the help; `aria-expanded` carries the state. */
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

/** The props of {@link HelpPanel}. */
export interface HelpPanelProps {
  readonly onDismiss: () => void;
}

/** The two-line explainer and its close control. */
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
