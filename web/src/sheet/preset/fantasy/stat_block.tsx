import { useCallback, useSyncExternalStore } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import { groupDisplayName } from '../../../engine/field_grouping';
import { message } from '../../../l10n/strings';
import { PencilIcon } from '../../../ui/icons';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue, textValue } from '../values';
import { FantasyAbilities } from './abilities';
import { ABILITY_KEYS, formatModifier } from './abilities_data';
import { ArmorClassShield } from './ac_breakdown';
import { INITIATIVE_PATH } from './armor_class';
import { FantasyAttacks } from './attacks';
import { HitPointsCard } from './hp_card';
import {
  abilityAbbreviationKey,
  isProficient,
  parseSkillBonus,
  saveProficiencyPath,
  saveTotalPath,
  skillBonusPath,
  skillProficiencyPath,
  skillRowKey,
  skillTotalPath,
  SKILL_KEYS,
  type SkillKey,
} from './saves_skills_data';
import { FantasySpells } from './spells';
import { parseInitiative, statBlockKindLabelKey } from './stat_block_data';
import './stat_block.css';

function useSkillLabel(skill: SkillKey): string {
  const { template } = usePresetContext();
  const skillsNode = template.fields.skills;
  const skillNode = skillsNode?.kind === 'group' ? skillsNode.children[skill] : undefined;
  return groupDisplayName(skill, skillNode?.kind === 'group' ? skillNode.label : undefined);
}

export function StatBlockHeader() {
  const { sheet } = usePresetContext();
  const name = textValue(usePresetValue('identity.characterName'));
  const commitName = usePresetCommit('identity.characterName');
  const nameLabel = usePresetLabel('identity.characterName');
  const kindKey = statBlockKindLabelKey(sheet.kind);

  return (
    <div className="lg-stat-block__header" data-testid="sheet-stat-block-header">
      <h1 className="lg-sheet__title" data-testid="sheet-name">
        <InlineEdit
          className="lg-stat-block__name"
          label={nameLabel}
          value={name}
          fallback={sheet.name}
          onCommit={commitName}
        />
      </h1>
      {kindKey !== undefined && (
        <span className="lg-stat-block__kind" data-testid="sheet-stat-kind">
          {message(kindKey)}
        </span>
      )}
    </div>
  );
}

function InitiativeTile() {
  const label = usePresetLabel(INITIATIVE_PATH);
  const initiative = numberValue(usePresetValue(INITIATIVE_PATH));
  const commitInitiative = usePresetCommit(INITIATIVE_PATH);

  return (
    <div className="lg-stat-block__stat" data-testid="sheet-stat-initiative">
      <span className="lg-stat-block__stat-label">{label}</span>
      <span className="lg-stat-block__stat-value">
        <RollableValue
          rowKey={INITIATIVE_PATH}
          sectionKey="combat"
          label={label}
          text={
            initiative === undefined ? message('sheet.play.noValue') : formatModifier(initiative)
          }
          allowRoll={initiative !== undefined}
          className="lg-stat-block__roll"
        />
        <InlineEdit
          className="lg-stat-block__edit"
          display={<PencilIcon className="lg-stat-block__pencil" />}
          label={label}
          value={initiative === undefined ? '' : String(initiative)}
          inputMode="numeric"
          onCommit={(text) => {
            const next = parseInitiative(text);
            if (next !== undefined) {
              commitInitiative(next);
            }
          }}
        />
      </span>
    </div>
  );
}

function SaveLine({ ability }: { readonly ability: (typeof ABILITY_KEYS)[number] }) {
  const totalPath = saveTotalPath(ability);
  const total = numberValue(usePresetValue(totalPath));
  const label = usePresetLabel(totalPath);

  return (
    <li className="lg-stat-block__prof" data-testid={`sheet-stat-save-${ability}`}>
      <span className="lg-stat-block__prof-name">{message(abilityAbbreviationKey(ability))}</span>
      <RollableValue
        rowKey={totalPath}
        sectionKey="saves"
        label={label}
        text={total === undefined ? message('sheet.play.noValue') : formatModifier(total)}
        allowRoll={total !== undefined}
        className="lg-stat-block__prof-value"
      />
    </li>
  );
}

function SkillLine({ skill }: { readonly skill: SkillKey }) {
  const bonusPath = skillBonusPath(skill);
  const totalPath = skillTotalPath(skill);
  const name = useSkillLabel(skill);
  const bonus = numberValue(usePresetValue(bonusPath));
  const total = numberValue(usePresetValue(totalPath));
  const bonusLabel = usePresetLabel(bonusPath);
  const commitBonus = usePresetCommit(bonusPath);

  return (
    <li className="lg-stat-block__prof" data-testid={`sheet-stat-skill-${skill}`}>
      <span className="lg-stat-block__prof-name">{name}</span>
      <InlineEdit
        className="lg-stat-block__prof-bonus"
        label={bonusLabel}
        value={bonus === undefined ? '' : formatModifier(bonus)}
        inputMode="numeric"
        onCommit={(text) => {
          const next = parseSkillBonus(text);
          if (next !== undefined) {
            commitBonus(next);
          }
        }}
      />
      <RollableValue
        rowKey={skillRowKey(skill)}
        sectionKey="skills"
        label={name}
        text={total === undefined ? message('sheet.play.noValue') : formatModifier(total)}
        allowRoll={total !== undefined}
        className="lg-stat-block__prof-value"
      />
    </li>
  );
}

function StatProficiencies() {
  const { store } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const saves = ABILITY_KEYS.filter((ability) =>
    isProficient(valueAtPath(state.data, saveProficiencyPath(ability))),
  );
  const skills = SKILL_KEYS.filter((skill) =>
    isProficient(valueAtPath(state.data, skillProficiencyPath(skill))),
  );

  if (saves.length === 0 && skills.length === 0) {
    return null;
  }

  return (
    <div className="lg-stat-block__profs" data-testid="sheet-stat-proficiencies">
      {saves.length > 0 && (
        <section
          className="lg-stat-block__prof-group"
          aria-label={message('sheet.play.saves')}
          data-testid="sheet-stat-saves"
        >
          <h2 className="lg-stat-block__heading">{message('sheet.play.saves')}</h2>
          <ul className="lg-stat-block__prof-list">
            {saves.map((ability) => (
              <SaveLine key={ability} ability={ability} />
            ))}
          </ul>
        </section>
      )}
      {skills.length > 0 && (
        <section
          className="lg-stat-block__prof-group"
          aria-label={message('sheet.play.skills')}
          data-testid="sheet-stat-skills"
        >
          <h2 className="lg-stat-block__heading">{message('sheet.play.skills')}</h2>
          <ul className="lg-stat-block__prof-list">
            {skills.map((skill) => (
              <SkillLine key={skill} skill={skill} />
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

export function FantasyStatBlock() {
  return (
    <div className="lg-stat-block" data-testid="sheet-stat-block">
      <div className="lg-stat-block__combat">
        <ArmorClassShield />
        <InitiativeTile />
        <div className="lg-stat-block__hp">
          <HitPointsCard />
        </div>
      </div>
      <FantasyAbilities />
      <StatProficiencies />
      <FantasyAttacks />
      <FantasySpells />
    </div>
  );
}
