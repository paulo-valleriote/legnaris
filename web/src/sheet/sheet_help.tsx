import type { Ref } from 'react';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { CloseIcon } from '../ui/icons';

export const HELP_PANEL_ID = 'lg-sheet-help';

export interface HelpToggleProps {
  readonly open: boolean;
  readonly onToggle: () => void;
  readonly buttonRef?: Ref<HTMLButtonElement>;
}

export function HelpToggle({ open, onToggle, buttonRef }: HelpToggleProps) {
  return (
    <Button
      ref={buttonRef}
      variant="ghost"
      size="sm"
      label={message('sheet.help')}
      ariaExpanded={open}
      ariaControls={HELP_PANEL_ID}
      onClick={onToggle}
    />
  );
}

export interface HelpPanelProps {
  readonly onDismiss: () => void;
  readonly boxed?: boolean;
}

export function HelpPanel({ onDismiss, boxed = false }: HelpPanelProps) {
  return (
    <div
      className="lg-sheet__help"
      id={HELP_PANEL_ID}
      role="region"
      aria-label={message('sheet.helpTitle')}
      data-testid="sheet-help"
    >
      <div className="lg-sheet__help-body">
        <p className="lg-sheet__help-text">
          {message(boxed ? 'sheet.helpBoxed' : 'sheet.helpDerived')}
        </p>
        <p className="lg-sheet__help-text">{message('sheet.helpPreview')}</p>
      </div>
      <Button
        variant="ghost"
        className="lg-sheet__help-dismiss"
        label={message('common.close')}
        onClick={onDismiss}
      >
        <CloseIcon className="lg-sheet__help-close" />
      </Button>
    </div>
  );
}
