import { sectionKeyOfRow } from '../../../engine/field_grouping';
import { message } from '../../../l10n/strings';
import { usePresetLabel, usePresetValue } from '../preset_context';
import { RollableValue } from '../rollable_value';
import type { PresetStatPill, PresetStatPillsPiece } from './types';
import './compose.css';

function displayValue(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return message('sheet.play.noValue');
  }
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return message('sheet.play.noValue');
}

function StatPill({ pill }: { readonly pill: PresetStatPill }) {
  const label = usePresetLabel(pill.path);
  const value = usePresetValue(pill.path);

  return (
    <li className="lg-compose-stat">
      <span className="lg-compose-stat__label">{label}</span>
      <RollableValue
        className="lg-compose-stat__value"
        rowKey={pill.path}
        sectionKey={sectionKeyOfRow(pill.path)}
        label={label}
        text={displayValue(value)}
      />
    </li>
  );
}

export function StatPills({ piece }: { readonly piece: PresetStatPillsPiece }) {
  if (piece.pills.length === 0) {
    return null;
  }

  return (
    <ul className="lg-compose-stats" data-testid="compose-stat-pills">
      {piece.pills.map((pill) => (
        <StatPill key={pill.path} pill={pill} />
      ))}
    </ul>
  );
}
