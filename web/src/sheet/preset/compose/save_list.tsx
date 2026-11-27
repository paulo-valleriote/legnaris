import { message, messageWith } from '../../../l10n/strings';
import {
  usePresetContext,
  usePresetCommit,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import { ProficiencyLegend } from './proficiency_legend';
import { formatSigned, isProficient, nextProficiency, rowDisplayName } from './scores_data';
import type { PresetSave, PresetSaveListPiece } from './types';
import { GroupTitle } from './group_title';
import './compose.css';

function SaveRow({ save, section }: { readonly save: PresetSave; readonly section: string }) {
  const total = numberValue(usePresetValue(save.total));
  const proficiency = usePresetValue(save.proficiency);
  const label = rowDisplayName(usePresetLabel(save.total), save.total);
  const commitProficiency = usePresetCommit(save.proficiency);
  const { editing } = usePresetContext();
  const proficient = isProficient(proficiency);
  const state = message(
    proficient ? 'sheet.play.proficiencyProficient' : 'sheet.play.proficiencyNotProficient',
  );

  return (
    <li className="lg-compose-save" data-testid={`compose-save-${save.total}`}>
      <button
        type="button"
        className="lg-compose-flag"
        aria-pressed={proficient}
        disabled={!editing}
        aria-label={messageWith('sheet.play.proficiencyForState', { label, state })}
        onClick={() => {
          commitProficiency(nextProficiency(proficiency));
        }}
      >
        <span className="lg-compose-flag__mark" aria-hidden="true" />
      </button>
      <span className="lg-compose-save__name" aria-hidden="true">
        {label}
      </span>
      <RollableValue
        className="lg-compose-save__total"
        rowKey={save.row ?? save.total}
        sectionKey={section}
        label={label}
        text={total === undefined ? message('sheet.play.noValue') : formatSigned(total)}
        allowRoll={total !== undefined}
      />
    </li>
  );
}

export function SaveList({ piece }: { readonly piece: PresetSaveListPiece }) {
  if (piece.saves.length === 0) {
    return null;
  }

  return (
    <div className="lg-compose-group">
      <div className="lg-compose-group__header">
        <GroupTitle>{message('sheet.play.saves')}</GroupTitle>
        <ProficiencyLegend testId="compose-saves-legend" />
      </div>
      <ul className="lg-compose-saves" data-testid="compose-save-list">
        {piece.saves.map((save) => (
          <SaveRow key={save.total} save={save} section={piece.section} />
        ))}
      </ul>
    </div>
  );
}
