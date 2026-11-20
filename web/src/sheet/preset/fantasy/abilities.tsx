import { groupDisplayName } from '../../../engine/field_grouping';
import { message } from '../../../l10n/strings';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import {
  ABILITY_KEYS,
  abilityModifierPath,
  abilityScorePath,
  formatModifier,
  parseAbilityScore,
  type AbilityKey,
} from './abilities_data';
import './abilities.css';
import './play_section.css';
import { PASSIVE_AWARENESS_PATH } from './stat_block_data';

function AbilityMedallion({ ability }: { readonly ability: AbilityKey }) {
  const scorePath = abilityScorePath(ability);
  const modifierPath = abilityModifierPath(ability);
  const score = numberValue(usePresetValue(scorePath));
  const modifier = numberValue(usePresetValue(modifierPath));
  const commitScore = usePresetCommit(scorePath);
  const scoreLabel = usePresetLabel(scorePath);
  const modifierLabel = usePresetLabel(modifierPath);
  const scoreSchema = usePresetSchema(scorePath);

  return (
    <li className="lg-ability" data-testid={`sheet-ability-${ability}`}>
      <span className="lg-ability__name">{scoreLabel}</span>
      <span className="lg-ability__medal">
        <RollableValue
          rowKey={scorePath}
          sectionKey="stats"
          label={modifierLabel}
          text={modifier === undefined ? message('sheet.play.noValue') : formatModifier(modifier)}
          allowRoll={modifier !== undefined}
          className="lg-ability__modifier"
        />
        <InlineEdit
          className="lg-ability__score"
          label={scoreLabel}
          path={scorePath}
          schema={scoreSchema}
          value={score === undefined ? '' : String(score)}
          inputMode="numeric"
          onCommit={(text) => {
            const next = parseAbilityScore(text);
            if (next !== undefined) {
              commitScore(next);
            }
          }}
        />
      </span>
    </li>
  );
}

function PassiveAwareness() {
  const passiveAwareness = numberValue(usePresetValue(PASSIVE_AWARENESS_PATH));
  const passiveLabel = usePresetLabel(PASSIVE_AWARENESS_PATH);

  return (
    <p className="lg-ability-passive" data-testid="sheet-play-passive">
      <span className="lg-ability-passive__value">
        {passiveAwareness === undefined ? message('sheet.play.noValue') : passiveAwareness}
      </span>{' '}
      <span className="lg-ability-passive__label">{passiveLabel}</span>
    </p>
  );
}

export function FantasyAbilities({ showPassive = true }: { readonly showPassive?: boolean }) {
  const { template } = usePresetContext();
  const stats = template.fields.stats;
  const sectionLabel = groupDisplayName('stats', stats?.kind === 'group' ? stats.label : undefined);

  return (
    <section className="lg-preset-abilities" aria-label={sectionLabel}>
      <h2 className="lg-play-section__title lg-preset-abilities__title">{sectionLabel}</h2>
      <ul className="lg-ability-grid">
        {ABILITY_KEYS.map((ability) => (
          <AbilityMedallion key={ability} ability={ability} />
        ))}
      </ul>
      {showPassive && <PassiveAwareness />}
    </section>
  );
}
