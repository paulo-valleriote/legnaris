import { message, messageWith } from '../../../l10n/strings';
import { usePresetCommit, usePresetLabel, usePresetValue } from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import { formatSigned, nextRank, rankLimit, rankValue } from './scores_data';
import type { PresetSkill, PresetSkillListPiece } from './types';
import './compose.css';

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
  const commitRank = usePresetCommit(skill.rank);
  const limit = rankLimit(max);
  const current = rankValue(rank, limit);
  const marks = Array.from({ length: limit }, (_, index) => index + 1);

  return (
    <li className="lg-compose-skill" data-testid={`compose-skill-${skill.rank}`}>
      {limit === 0 ? null : (
        <button
          type="button"
          className="lg-compose-rank"
          aria-label={messageWith('preset.skill.rank', {
            label: rankLabel,
            count: current,
            max: limit,
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
        {rankLabel}
      </span>
      {skill.ability === undefined ? null : (
        <span className="lg-compose-skill__ability">{skill.ability}</span>
      )}
      <RollableValue
        className="lg-compose-skill__total"
        rowKey={skill.total}
        sectionKey={section}
        label={totalLabel}
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
  return (
    <ul className="lg-compose-skills" data-testid="compose-skill-list">
      {skills.map((skill) => (
        <SkillRow key={skill.rank} skill={skill} section={section} max={max} />
      ))}
    </ul>
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
