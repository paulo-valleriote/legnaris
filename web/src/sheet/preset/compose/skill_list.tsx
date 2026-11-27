import { message, messageWith, type MessageKey } from '../../../l10n/strings';
import {
  usePresetContext,
  usePresetCommit,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import { ProficiencyLegend } from './proficiency_legend';
import {
  formatSigned,
  nextRank,
  rankLimit,
  rankProficiency,
  rankValue,
  rowDisplayName,
  type RankProficiency,
} from './scores_data';
import type { PresetSkill, PresetSkillListPiece } from './types';
import { GroupTitle } from './group_title';
import './compose.css';

const proficiencyCopy: Record<RankProficiency, MessageKey> = {
  none: 'sheet.play.proficiencyNotProficient',
  proficient: 'sheet.play.proficiencyProficient',
  expertise: 'sheet.play.proficiencyExpertise',
};

function SkillRow({
  skill,
  section,
  max,
}: {
  readonly skill: PresetSkill;
  readonly section: string;
  readonly max: number;
}) {
  const rank = usePresetValue(skill.rank);
  const total = numberValue(usePresetValue(skill.total));
  const rankLabel = usePresetLabel(skill.rank);
  const totalLabel = usePresetLabel(skill.total);
  const name = rowDisplayName(rankLabel, skill.rank);
  const rollLabel = rowDisplayName(totalLabel, skill.total);
  const commitRank = usePresetCommit(skill.rank);
  const { editing, readOnly } = usePresetContext();
  const limit = rankLimit(max);
  const current = rankValue(rank, limit);
  const marks = Array.from({ length: limit }, (_, index) => index + 1);
  const state = message(proficiencyCopy[rankProficiency(current)]);

  return (
    <li className="lg-compose-skill" data-testid={`compose-skill-${skill.rank}`}>
      {limit === 0 ? null : readOnly ? (
        <span
          className="lg-compose-rank"
          aria-label={messageWith('preset.skill.rank', {
            label: name,
            count: current,
            max: limit,
            state,
          })}
        >
          {marks.map((index) => (
            <span
              key={index}
              className="lg-compose-rank__mark"
              data-filled={index <= current ? '' : undefined}
              aria-hidden="true"
            />
          ))}
        </span>
      ) : (
        <button
          type="button"
          className="lg-compose-rank"
          disabled={!editing}
          aria-label={messageWith('preset.skill.rank', {
            label: name,
            count: current,
            max: limit,
            state,
          })}
          onClick={() => {
            commitRank(nextRank(current, limit));
          }}
        >
          {marks.map((index) => (
            <span
              key={index}
              className="lg-compose-rank__mark"
              data-filled={index <= current ? '' : undefined}
              aria-hidden="true"
            />
          ))}
        </button>
      )}
      <span className="lg-compose-skill__name" aria-hidden="true">
        {name}
      </span>
      {skill.ability === undefined ? null : (
        <span className="lg-compose-skill__ability">{skill.ability}</span>
      )}
      <RollableValue
        className="lg-compose-skill__total"
        rowKey={skill.row ?? skill.total}
        sectionKey={section}
        label={rollLabel}
        text={total === undefined ? message('sheet.play.noValue') : formatSigned(total)}
        allowRoll={total !== undefined}
      />
    </li>
  );
}

function SkillsWithMaxPath({
  skills,
  section,
  maxPath,
}: {
  readonly skills: readonly PresetSkill[];
  readonly section: string;
  readonly maxPath: string;
}) {
  const max = numberValue(usePresetValue(maxPath)) ?? 0;

  return <SkillsBody skills={skills} section={section} max={max} />;
}

function SkillsBody({
  skills,
  section,
  max,
}: {
  readonly skills: readonly PresetSkill[];
  readonly section: string;
  readonly max: number;
}) {
  const limit = rankLimit(max);

  return (
    <div className="lg-compose-group lg-compose-group--ranked">
      <div className="lg-compose-group__header">
        <GroupTitle>{message('sheet.play.skills')}</GroupTitle>
        {limit === 0 ? null : (
          <ProficiencyLegend expertise={limit >= 2} testId="compose-skills-legend" />
        )}
      </div>
      <ul className="lg-compose-skills" data-testid="compose-skill-list">
        {skills.map((skill) => (
          <SkillRow key={skill.rank} skill={skill} section={section} max={max} />
        ))}
      </ul>
    </div>
  );
}

export function SkillList({ piece }: { readonly piece: PresetSkillListPiece }) {
  if (piece.skills.length === 0) {
    return null;
  }
  if (typeof piece.max === 'number') {
    return <SkillsBody skills={piece.skills} section={piece.section} max={piece.max} />;
  }
  return <SkillsWithMaxPath skills={piece.skills} section={piece.section} maxPath={piece.max} />;
}
