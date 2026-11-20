import { message } from '../l10n/strings';
import { Button } from '../ui/button';

export type SheetMode = 'play' | 'edit';

export interface SheetModeToggleProps {
  readonly mode: SheetMode;
  readonly disabled?: boolean;
  readonly onChange: (mode: SheetMode) => void;
}

export function SheetModeToggle({ mode, disabled = false, onChange }: SheetModeToggleProps) {
  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        disabled={disabled}
        ariaPressed={mode === 'play'}
        label={message('sheet.mode.play')}
        onClick={() => onChange('play')}
      />
      <Button
        variant="ghost"
        size="sm"
        disabled={disabled}
        ariaPressed={mode === 'edit'}
        label={message('sheet.mode.edit')}
        onClick={() => onChange('edit')}
      />
    </>
  );
}
