import { Fragment, useCallback, useSyncExternalStore } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import { groupDisplayName } from '../../../engine/field_grouping';
import { message, messageWith } from '../../../l10n/strings';
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
import { INITIATIVE_PATH, SPEED_PATH, parseInitiative } from './armor_class';
import { FantasyAttacks } from './attacks';
import { HitPointsCard } from './hp_card';
import {
  abilityAbbreviationKey,
  isProficient,
  saveProficiencyPath,
  saveTotalPath,
  skillProficiencyPath,
  skillRowKey,
  skillTotalPath,
  SKILL_KEYS,
  type SkillKey,
} from './saves_skills_data';
import { FantasySpells } from './spells';
import {
  PASSIVE_AWARENESS_PATH,
  statBlockAbilityWord,
  statBlockKindLabelKey,
  statBlockSubtitleParts,
  statMovementParts,
  statProseText,
} from './stat_block_data';
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
  const ancestry = textValue(usePresetValue('identity.ancestry'));
  const origin = textValue(usePresetValue('identity.origin'));
  const level = numberValue(usePresetValue('identity.level'));
  const commitName = usePresetCommit('identity.characterName');
  const nameLabel = usePresetLabel('identity.characterName');
  const kindKey = statBlockKindLabelKey(sheet.kind);
  const subtitleParts = statBlockSubtitleParts({ ancestry, origin, level }, (count) =>
    messageWith('sheet.play.level', { count }),
  );

  return (
    <div className="lg-stat-block__header" data-testid="sheet-stat-block-header">
      <div className="lg-stat-block__heading">
        <h1 className="lg-sheet__title" data-testid="sheet-name">
          <InlineEdit
            className="lg-stat-block__name"
            label={nameLabel}
            value={name}
            fallback={sheet.name}
            onCommit={commitName}
          />
        </h1>
        {subtitleParts.length > 0 && (
          <p className="lg-stat-block__subtitle" data-testid="sheet-stat-subtitle">
            {subtitleParts.map((part, index) => (
              <Fragment key={`${part}-${index}`}>
                {index > 0 && (
                  <span className="lg-stat-block__separator" data-separator aria-hidden="true">
                    {' · '}
                  </span>
                )}
                {part}
              </Fragment>
            ))}
          </p>
        )}
      </div>
      {kindKey !== undefined && (
        <span className="lg-stat-block__kind" data-testid="sheet-stat-kind" data-kind={sheet.kind}>
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

function SaveItem({ ability }: { readonly ability: (typeof ABILITY_KEYS)[number] }) {
  const totalPath = saveTotalPath(ability);
  const total = numberValue(usePresetValue(totalPath));
  const label = usePresetLabel(totalPath);

  return (
    <span className="lg-stat-block__line-item" data-testid={`sheet-stat-save-${ability}`}>
      <span className="lg-stat-block__line-name" aria-hidden="true">
        {statBlockAbilityWord(message(abilityAbbreviationKey(ability)))}
      </span>{' '}
      <RollableValue
        rowKey={totalPath}
        sectionKey="saves"
        label={label}
        text={total === undefined ? message('sheet.play.noValue') : formatModifier(total)}
        allowRoll={total !== undefined}
      />
    </span>
  );
}

function SkillItem({ skill }: { readonly skill: SkillKey }) {
  const totalPath = skillTotalPath(skill);
  const name = useSkillLabel(skill);
  const total = numberValue(usePresetValue(totalPath));

  return (
    <span className="lg-stat-block__line-item" data-testid={`sheet-stat-skill-${skill}`}>
      <span className="lg-stat-block__line-name">{name}</span>{' '}
      <RollableValue
        rowKey={skillRowKey(skill)}
        sectionKey="skills"
        label={name}
        text={total === undefined ? message('sheet.play.noValue') : formatModifier(total)}
        allowRoll={total !== undefined}
      />
    </span>
  );
}

function StatProse() {
  const { store } = usePresetContext();
  const speedLabel = usePresetLabel(SPEED_PATH);
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const saves = ABILITY_KEYS.filter((ability) =>
    isProficient(valueAtPath(state.data, saveProficiencyPath(ability))),
  );
  const skills = SKILL_KEYS.filter((skill) =>
    isProficient(valueAtPath(state.data, skillProficiencyPath(skill))),
  );
  const movement = statProseText(
    statMovementParts(
      numberValue(valueAtPath(state.data, SPEED_PATH)),
      numberValue(valueAtPath(state.data, PASSIVE_AWARENESS_PATH)),
      {
        speed: speedLabel,
        passive: message('sheet.play.passiveShort'),
        feet: (count) => messageWith('sheet.play.feet', { count }),
      },
    ),
  );

  if (saves.length === 0 && skills.length === 0 && movement === undefined) {
    return null;
  }

  return (
    <div className="lg-stat-block__prose" data-testid="sheet-stat-prose">
      {saves.length > 0 && (
        <p className="lg-stat-block__line" data-testid="sheet-stat-saves">
          <span className="lg-stat-block__line-term">{message('sheet.play.savesShort')}</span>{' '}
          {saves.map((ability, index) => (
            <Fragment key={ability}>
              {index > 0 && (
                <span className="lg-stat-block__separator" data-separator aria-hidden="true">
                  {' \u00b7 '}
                </span>
              )}
              <SaveItem ability={ability} />
            </Fragment>
          ))}
        </p>
      )}
      {skills.length > 0 && (
        <p className="lg-stat-block__line" data-testid="sheet-stat-skills">
          <span className="lg-stat-block__line-term">{message('sheet.play.skills')}</span>{' '}
          {skills.map((skill, index) => (
            <Fragment key={skill}>
              {index > 0 && (
                <span className="lg-stat-block__separator" data-separator aria-hidden="true">
                  {' \u00b7 '}
                </span>
              )}
              <SkillItem skill={skill} />
            </Fragment>
          ))}
        </p>
      )}
      {movement !== undefined && (
        <p className="lg-stat-block__line" data-testid="sheet-stat-speed">
          {movement}
        </p>
      )}
    </div>
  );
}

export function FantasyStatBlock() {
  return (
    <div className="lg-stat-block" data-testid="sheet-stat-block">
      <StatBlockHeader />
      <div className="lg-stat-block__combat" data-testid="sheet-stat-band">
        <ArmorClassShield />
        <InitiativeTile />
        <div className="lg-stat-block__hp">
          <HitPointsCard />
        </div>
      </div>
      <FantasyAbilities showPassive={false} />
      <StatProse />
      <FantasyAttacks />
      <FantasySpells variant="compact" />
    </div>
  );
}
