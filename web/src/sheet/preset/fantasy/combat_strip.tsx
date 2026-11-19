import { message } from '../../../l10n/strings';
import { InlineEdit } from '../../inline_edit';
import { usePresetCommit, usePresetLabel, usePresetValue } from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import { ArmorClassShield } from './ac_breakdown';
import { formatModifier } from './abilities_data';
import { INITIATIVE_PATH, SPEED_PATH, parseSpeed } from './armor_class';
import { HitPointsCard } from './hp_card';
import './combat_strip.css';

function InitiativeTile() {
  const label = usePresetLabel(INITIATIVE_PATH);
  const initiative = numberValue(usePresetValue(INITIATIVE_PATH));

  return (
    <div className="lg-combat-stat" data-testid="sheet-initiative">
      <span className="lg-combat-stat__label">{label}</span>
      <RollableValue
        rowKey={INITIATIVE_PATH}
        sectionKey="combat"
        label={label}
        text={initiative === undefined ? message('sheet.play.noValue') : formatModifier(initiative)}
        allowRoll={initiative !== undefined}
        className="lg-combat-stat__value"
      />
    </div>
  );
}

function SpeedTile() {
  const label = usePresetLabel(SPEED_PATH);
  const speed = numberValue(usePresetValue(SPEED_PATH));
  const commitSpeed = usePresetCommit(SPEED_PATH);

  return (
    <div className="lg-combat-stat" data-testid="sheet-speed">
      <span className="lg-combat-stat__label">{label}</span>
      <InlineEdit
        className="lg-combat-stat__value"
        label={label}
        value={speed === undefined ? '' : String(speed)}
        inputMode="numeric"
        onCommit={(text) => {
          const next = parseSpeed(text);
          if (next !== undefined) {
            commitSpeed(next);
          }
        }}
      />
    </div>
  );
}

export function CombatStrip() {
  return (
    <div className="lg-combat-strip" data-testid="sheet-combat-strip">
      <ArmorClassShield />
      <InitiativeTile />
      <SpeedTile />
      <HitPointsCard />
    </div>
  );
}
