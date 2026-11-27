import { message } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { usePresetContext } from '../preset_context';
import { MODE_KEYS, MODE_ORDER } from '../../roll_mode';
import type { PresetRollModePiece } from './types';
import './compose.css';

export function RollModeBar({ piece }: { readonly piece: PresetRollModePiece }) {
  const { rollMode, setRollMode, readOnly } = usePresetContext();

  if (readOnly) {
    return null;
  }

  return (
    <div
      className="lg-compose-roll-mode"
      role="group"
      aria-label={message('preset.rollMode.group')}
      data-testid="compose-roll-mode"
      data-piece-key={piece.key}
    >
      {MODE_ORDER.map((mode) => (
        <Button
          key={mode}
          variant="secondary"
          size="sm"
          label={message(MODE_KEYS[mode])}
          ariaPressed={rollMode === mode}
          onClick={() => setRollMode(mode)}
        />
      ))}
    </div>
  );
}
