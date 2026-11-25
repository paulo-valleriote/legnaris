import { message, messageWith } from '../../../l10n/strings';
import { usePresetValue } from '../preset_context';
import { numberValue } from '../values';
import { GroupTitle } from './group_title';
import { PipsWithMaxPath } from './pips';
import type { PresetSpellSlotsPiece } from './types';
import './compose.css';

export function SpellSlots({ piece }: { readonly piece: PresetSpellSlotsPiece }) {
  const highest = piece.levels.at(-1);
  const highestMax = numberValue(usePresetValue(highest?.max ?? piece.key)) ?? 0;
  const moreToCome = highest !== undefined && highestMax <= 0;

  if (piece.levels.length === 0 && piece.pact === undefined) {
    return null;
  }

  return (
    <div className="lg-compose-group lg-compose-slots" data-testid="compose-spell-slots">
      <div className="lg-compose-group__header">
        <GroupTitle>{message('sheet.play.spellSlots')}</GroupTitle>
      </div>
      <div className="lg-compose-slots__rows">
        {piece.levels.map((level) => (
          <PipsWithMaxPath
            key={level.count}
            countPath={level.count}
            maxPath={level.max}
            label={messageWith('sheet.play.level', { count: level.level })}
          />
        ))}
        {piece.pact === undefined ? null : (
          <PipsWithMaxPath countPath={piece.pact.count} maxPath={piece.pact.max} />
        )}
      </div>
      {moreToCome ? <p className="lg-compose-slots__note">{message('preset.slots.more')}</p> : null}
    </div>
  );
}
