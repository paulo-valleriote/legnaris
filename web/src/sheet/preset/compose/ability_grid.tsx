import { message } from '../../../l10n/strings';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import { formatSigned, parseScore } from './scores_data';
import type { PresetAbility, PresetAbilityGridPiece } from './types';
import './compose.css';

function AbilityCell({
  ability,
  section,
}: {
  readonly ability: PresetAbility;
  readonly section: string;
}) {
  const score = numberValue(usePresetValue(ability.score));
  const modifier = numberValue(usePresetValue(ability.modifier));
  const scoreLabel = usePresetLabel(ability.score);
  const modifierLabel = usePresetLabel(ability.modifier);
  const scoreSchema = usePresetSchema(ability.score);
  const commitScore = usePresetCommit(ability.score);

  return (
    <li className="lg-compose-ability" data-testid={`compose-ability-${ability.score}`}>
      <span className="lg-compose-ability__name">{scoreLabel}</span>
      <RollableValue
        className="lg-compose-ability__modifier"
        rowKey={ability.row ?? ability.modifier}
        sectionKey={section}
        label={modifierLabel}
        text={modifier === undefined ? message('sheet.play.noValue') : formatSigned(modifier)}
        allowRoll={modifier !== undefined}
      />
      <InlineEdit
        className="lg-compose-ability__score"
        label={scoreLabel}
        path={ability.score}
        schema={scoreSchema}
        value={score === undefined ? '' : String(score)}
        inputMode="numeric"
        onCommit={(text) => {
          const next = parseScore(text);
          if (next !== undefined) {
            commitScore(next);
          }
        }}
      />
    </li>
  );
}

export function AbilityGrid({ piece }: { readonly piece: PresetAbilityGridPiece }) {
  if (piece.abilities.length === 0) {
    return null;
  }

  return (
    <ul className="lg-compose-abilities" data-testid="compose-ability-grid">
      {piece.abilities.map((ability) => (
        <AbilityCell key={ability.score} ability={ability} section={piece.section} />
      ))}
    </ul>
  );
}
