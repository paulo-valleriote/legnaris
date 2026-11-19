import { groupDisplayName } from '../../../engine/field_grouping';
import { message, messageWith } from '../../../l10n/strings';
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
import { ABILITY_KEYS, formatModifier, type AbilityKey } from './abilities_data';
import {
  abilityAbbreviationKey,
  isProficient,
  nextProficiency,
  parseSkillBonus,
  saveProficiencyPath,
  saveTotalPath,
  skillAbility,
  skillBonusPath,
  skillProficiencyPath,
  skillRowKey,
  skillTotalPath,
  SKILL_KEYS,
  type SkillKey,
} from './saves_skills_data';
import './saves_skills.css';

function ProficiencyDot({ path, label }: { readonly path: string; readonly label: string }) {
  const { store } = usePresetContext();
  const value = usePresetValue(path);
  const commit = usePresetCommit(path);

  return (
    <button
      type="button"
      className="lg-prof-dot"
      aria-pressed={isProficient(value)}
      aria-label={messageWith('sheet.play.proficiencyFor', { label })}
      data-edit-path={path}
      onClick={() => {
        commit(nextProficiency(value));
        void store.preview();
      }}
    >
      <span className="lg-prof-dot__mark" aria-hidden="true" />
    </button>
  );
}

function SaveRow({ ability }: { readonly ability: AbilityKey }) {
  const totalPath = saveTotalPath(ability);
  const total = numberValue(usePresetValue(totalPath));
  const label = usePresetLabel(totalPath);

  return (
    <li className="lg-save" data-testid={`sheet-save-${ability}`}>
      <ProficiencyDot path={saveProficiencyPath(ability)} label={label} />
      <span className="lg-save__name">{label}</span>
      <RollableValue
        rowKey={totalPath}
        sectionKey="saves"
        label={label}
        text={total === undefined ? message('sheet.play.noValue') : formatModifier(total)}
        allowRoll={total !== undefined}
        className="lg-save__total"
      />
    </li>
  );
}

function useSkillName(skill: SkillKey): string {
  const { template } = usePresetContext();
  const skillsNode = template.fields.skills;
  const skillNode = skillsNode?.kind === 'group' ? skillsNode.children[skill] : undefined;
  return groupDisplayName(skill, skillNode?.kind === 'group' ? skillNode.label : undefined);
}

function SkillRow({ skill }: { readonly skill: SkillKey }) {
  const bonusPath = skillBonusPath(skill);
  const totalPath = skillTotalPath(skill);
  const name = useSkillName(skill);
  const bonus = numberValue(usePresetValue(bonusPath));
  const total = numberValue(usePresetValue(totalPath));
  const bonusLabel = usePresetLabel(bonusPath);
  const bonusSchema = usePresetSchema(bonusPath);
  const commitBonus = usePresetCommit(bonusPath);

  return (
    <li className="lg-skill" data-testid={`sheet-skill-${skill}`}>
      <ProficiencyDot path={skillProficiencyPath(skill)} label={name} />
      <span className="lg-skill__name">{name}</span>
      <span className="lg-skill__ability">
        {message(abilityAbbreviationKey(skillAbility(skill)))}
      </span>
      <InlineEdit
        className="lg-skill__bonus"
        label={bonusLabel}
        path={bonusPath}
        schema={bonusSchema}
        value={bonus === undefined ? '' : String(bonus)}
        display={bonus === undefined ? undefined : formatModifier(bonus)}
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
        className="lg-skill__total"
      />
    </li>
  );
}

export function FantasySaves() {
  return (
    <section
      className="lg-preset-saves"
      aria-label={message('sheet.play.saves')}
      data-testid="sheet-play-saves"
    >
      <h2 className="lg-preset-saves__title">{message('sheet.play.saves')}</h2>
      <ul className="lg-preset-saves__list">
        {ABILITY_KEYS.map((ability) => (
          <SaveRow key={ability} ability={ability} />
        ))}
      </ul>
    </section>
  );
}

export function FantasySkills() {
  return (
    <section
      className="lg-preset-skills"
      aria-label={message('sheet.play.skills')}
      data-testid="sheet-play-skills"
    >
      <h2 className="lg-preset-skills__title">{message('sheet.play.skills')}</h2>
      <ul className="lg-preset-skills__list">
        {SKILL_KEYS.map((skill) => (
          <SkillRow key={skill} skill={skill} />
        ))}
      </ul>
    </section>
  );
}
