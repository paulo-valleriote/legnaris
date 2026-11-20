import { useRef } from 'react';
import { createPortal } from 'react-dom';
import type { RollMode } from '../dice/roll';
import { message, messageWith, type MessageKey } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';

const MODE_ORDER: readonly RollMode[] = ['normal', 'advantage', 'disadvantage'];

const MODE_KEYS: Record<RollMode, MessageKey> = {
  normal: 'dice.mode.normal',
  advantage: 'dice.mode.advantage',
  disadvantage: 'dice.mode.disadvantage',
};

export function rollModeFromClick(event: {
  readonly shiftKey: boolean;
  readonly altKey: boolean;
}): RollMode {
  if (event.shiftKey) {
    return 'advantage';
  }
  return event.altKey ? 'disadvantage' : 'normal';
}

export function opensRollModeMenu(event: {
  readonly key: string;
  readonly shiftKey: boolean;
}): boolean {
  return event.key === 'ContextMenu' || (event.key === 'F10' && event.shiftKey);
}

export interface RollModeDialogProps {
  readonly label: string;
  readonly onChoose: (mode: RollMode) => void;
  readonly onClose: () => void;
}

export function RollModeDialog({ label, onChoose, onClose }: RollModeDialogProps) {
  const firstOption = useRef<HTMLButtonElement>(null);
  return createPortal(
    <Dialog
      open
      title={messageWith('dice.modeTitle', { label })}
      onClose={onClose}
      initialFocusRef={firstOption}
    >
      <div className="lg-roll-mode">
        {MODE_ORDER.map((mode, index) => (
          <Button
            key={mode}
            ref={index === 0 ? firstOption : undefined}
            variant="secondary"
            size="md"
            fullWidth
            label={message(MODE_KEYS[mode])}
            onClick={() => onChoose(mode)}
          />
        ))}
      </div>
    </Dialog>,
    document.body,
  );
}
