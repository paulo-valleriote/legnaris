import { message, messageWith } from '../../../l10n/strings';
import { PencilIcon } from '../../../ui/icons';
import { DieIcon } from '../../die_icon';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue } from '../values';
import { ArmorClassShield } from './ac_breakdown';
import { formatModifier } from './abilities_data';
import { INITIATIVE_PATH, SPEED_PATH, parseInitiative, parseSpeed } from './armor_class';
import { HitPointsCard } from './hp_card';
import './combat_strip.css';

function InitiativeTile() {
  const label = usePresetLabel(INITIATIVE_PATH);
  const initiative = numberValue(usePresetValue(INITIATIVE_PATH));
  const schema = usePresetSchema(INITIATIVE_PATH);
  const commitInitiative = usePresetCommit(INITIATIVE_PATH);
  const initiativeText =
    initiative === undefined ? message('sheet.play.noValue') : formatModifier(initiative);
  const initiativeRoll =
    initiative === undefined
      ? undefined
      : messageWith('sheet.play.initiativeRoll', { value: initiativeText });

  return (
    <div className="lg-combat-stat" data-testid="sheet-initiative">
      <span className="lg-combat-stat__label">{label}</span>
      <span className="lg-combat-stat__value-row">
        <RollableValue
          rowKey={INITIATIVE_PATH}
          sectionKey="combat"
          label={label}
          text={initiativeText}
          allowRoll={initiative !== undefined}
          className="lg-combat-stat__value"
          accessibleName={initiativeRoll}
          icon={<DieIcon shape="d20" />}
        />
        <InlineEdit
          className="lg-combat-stat__edit"
          display={<PencilIcon className="lg-combat-stat__pencil" />}
          label={label}
          editLabel={message('sheet.play.editInitiative')}
          path={INITIATIVE_PATH}
          schema={schema}
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

function SpeedTile() {
  const label = usePresetLabel(SPEED_PATH);
  const speed = numberValue(usePresetValue(SPEED_PATH));
  const schema = usePresetSchema(SPEED_PATH);
  const commitSpeed = usePresetCommit(SPEED_PATH);

  return (
    <div className="lg-combat-stat" data-testid="sheet-speed">
      <span className="lg-combat-stat__label">{label}</span>
      <InlineEdit
        className="lg-combat-stat__value"
        label={label}
        path={SPEED_PATH}
        schema={schema}
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
    <div className="lg-combat-strip" data-testid="sheet-combat-strip" data-vitals-anchor="">
      <ArmorClassShield />
      <InitiativeTile />
      <SpeedTile />
      <HitPointsCard />
    </div>
  );
}
