import { useId, useState } from 'react';
import { message } from '../../../l10n/strings';
import { usePresetCommit, usePresetLabel, usePresetValue } from '../preset_context';
import { numberValue } from '../values';
import { conditionIsOn, conditionLabelParts } from './condition_label';
import { Pips } from './pips';
import type { PresetConditionFlag, PresetConditionsPiece } from './types';
import { GroupTitle } from './group_title';
import './compose.css';

function ConditionChip({
  flag,
  collapsed,
}: {
  readonly flag: PresetConditionFlag;
  readonly collapsed: boolean;
}) {
  const label = usePresetLabel(flag.path);
  const value = usePresetValue(flag.path);
  const commit = usePresetCommit(flag.path);
  const ruleId = useId();
  const on = conditionIsOn(value);
  const { name, rule } = conditionLabelParts(label);

  return (
    <li className="lg-compose-conditions__item" hidden={collapsed && !on}>
      <button
        type="button"
        className="lg-compose-condition"
        aria-pressed={on}
        aria-label={label}
        aria-describedby={rule === undefined ? undefined : ruleId}
        title={rule}
        onClick={() => commit(on ? 0 : 1)}
      >
        <span aria-hidden="true">{name}</span>
      </button>
      {rule === undefined ? null : (
        <span id={ruleId} className="lg-visually-hidden">
          {rule}
        </span>
      )}
    </li>
  );
}

function ExhaustionPenalty({ path }: { readonly path: string }) {
  const label = usePresetLabel(path);
  const value = usePresetValue(path);
  const shown = numberValue(value);

  return (
    <p className="lg-compose-conditions__penalty">
      <span className="lg-compose-conditions__penalty-label">{label}</span>
      <span className="lg-compose-conditions__penalty-value">
        {shown === undefined ? message('sheet.play.noValue') : String(shown)}
      </span>
    </p>
  );
}

export function Conditions({ piece }: { readonly piece: PresetConditionsPiece }) {
  const [expanded, setExpanded] = useState(false);
  const listId = useId();

  if (piece.conditions.length === 0 && piece.exhaustion === undefined) {
    return null;
  }

  return (
    <div className="lg-compose-conditions" data-testid="compose-conditions">
      {piece.conditions.length === 0 ? null : (
        <>
          <div className="lg-compose-group__header">
            <GroupTitle>{message('preset.conditions.title')}</GroupTitle>
            <button
              type="button"
              className="lg-compose-conditions__toggle"
              aria-expanded={expanded}
              aria-controls={listId}
              data-testid="compose-conditions-toggle"
              onClick={() => setExpanded((open) => !open)}
            >
              {message(expanded ? 'preset.conditions.hide' : 'preset.conditions.show')}
            </button>
          </div>
          {expanded ? null : (
            <p className="lg-compose-conditions__none">{message('preset.conditions.none')}</p>
          )}
          <ul className="lg-compose-conditions__list" id={listId}>
            {piece.conditions.map((flag) => (
              <ConditionChip key={flag.path} flag={flag} collapsed={!expanded} />
            ))}
          </ul>
        </>
      )}
      {piece.exhaustion === undefined ? null : (
        <div className="lg-compose-conditions__exhaustion">
          <Pips
            piece={{
              kind: 'pips',
              key: `${piece.key}.exhaustion`,
              count: piece.exhaustion.level,
              max: piece.exhaustion.max,
            }}
          />
          {piece.exhaustion.penalty === undefined ? null : (
            <ExhaustionPenalty path={piece.exhaustion.penalty} />
          )}
        </div>
      )}
    </div>
  );
}
