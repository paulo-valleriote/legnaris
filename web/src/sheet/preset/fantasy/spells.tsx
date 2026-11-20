import { useCallback, useId, useMemo, useSyncExternalStore } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import type { SheetField } from '../../../engine/field_grouping';
import { fieldDisplayName, groupDisplayName, lastSegment } from '../../../engine/field_grouping';
import { message, messageWith } from '../../../l10n/strings';
import { DiceProvider } from '../../dice_row';
import type { DiceContextValue } from '../../dice_row';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetField,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue, textValue } from '../values';
import { formatLevelRanges } from './hidden_levels';
import { parseClampedInt, UNBOUNDED } from './parse_number';
import { recoverSlot, slotPips, slotPool, spendSlot } from './slot_pips';
import type { SlotPool } from './slot_pips';
import './play_section.css';
import './spells.css';

const SPELLCASTING_ABILITY_GROUP = 'spellcastingAbility';

const SPELL_ATTACK_PATH = 'spells.attackBonus';

const SPELL_SAVE_DC_PATH = 'spells.saveDC';

const SPELL_ATTACK_DICE = '1d20 + @';

const SLOT_LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

function slotPath(level: number, leaf: 'total' | 'expended'): string {
  return `spells.slot${level}.${leaf}`;
}

function numberOrMissing(value: unknown): string {
  const number = numberValue(value);
  return number === undefined ? message('sheet.play.noValue') : String(number);
}

function signedValue(value: unknown): string {
  const number = numberValue(value);
  if (number === undefined) {
    return message('sheet.play.noValue');
  }
  return number >= 0 ? `+${number}` : String(number);
}

interface AbilityOption {
  readonly value: string;
  readonly label: string;
}

function abilityOptions(fields: ReadonlyMap<string, SheetField>): AbilityOption[] {
  return [...fields.values()]
    .filter((field) => field.schema.exclusiveGroup === SPELLCASTING_ABILITY_GROUP)
    .map((field) => ({
      value: field.path,
      label:
        field.schema.optionLabel?.trim() || fieldDisplayName(lastSegment(field.path), field.schema),
    }));
}

function SpellcastingAbility() {
  const { store, fields } = usePresetContext();
  const selectId = useId();
  const options = useMemo(() => abilityOptions(fields), [fields]);
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);

  if (options.length === 0) {
    return null;
  }

  const selected = options.find((option) => {
    const value = numberValue(valueAtPath(state.data, option.value));
    return value !== undefined && value !== 0;
  })?.value;
  const label = groupDisplayName(SPELLCASTING_ABILITY_GROUP);

  return (
    <div className="lg-spells__ability">
      <label className="lg-spells__label" htmlFor={selectId}>
        {label}
      </label>
      <select
        id={selectId}
        className="lg-spells__ability-select"
        value={selected ?? ''}
        onChange={(event) =>
          store.setExclusive(
            event.target.value,
            options.map((option) => option.value),
          )
        }
      >
        {selected === undefined && (
          <option value="" disabled>
            {message('engine.none')}
          </option>
        )}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

function SpellAttackValue({
  label,
  attackBonus,
}: {
  readonly label: string;
  readonly attackBonus: unknown;
}) {
  const { dice } = usePresetContext();
  const value = useMemo<DiceContextValue>(
    () => ({
      ...dice,
      defaults: {
        sectionDice: dice.defaults.sectionDice,
        rowDice: { ...dice.defaults.rowDice, [SPELL_ATTACK_PATH]: SPELL_ATTACK_DICE },
      },
    }),
    [dice],
  );

  return (
    <DiceProvider value={value}>
      <RollableValue
        rowKey={SPELL_ATTACK_PATH}
        sectionKey="spells"
        label={label}
        text={signedValue(attackBonus)}
        allowRoll={numberValue(attackBonus) !== undefined}
        className="lg-spells__rollable"
      />
    </DiceProvider>
  );
}

function SpellStats() {
  const { fields } = usePresetContext();
  const attackLabel = usePresetLabel(SPELL_ATTACK_PATH);
  const saveLabel = usePresetLabel(SPELL_SAVE_DC_PATH);
  const attackBonus = usePresetValue(SPELL_ATTACK_PATH);
  const saveDC = usePresetValue(SPELL_SAVE_DC_PATH);

  if (!fields.has(SPELL_ATTACK_PATH) && !fields.has(SPELL_SAVE_DC_PATH)) {
    return null;
  }

  return (
    <>
      {fields.has(SPELL_ATTACK_PATH) && (
        <div className="lg-spells__stat" data-testid="sheet-spell-attack">
          <span className="lg-spells__label">{attackLabel}</span>
          <span className="lg-spells__stat-value">
            <SpellAttackValue label={attackLabel} attackBonus={attackBonus} />
          </span>
        </div>
      )}
      {fields.has(SPELL_SAVE_DC_PATH) && (
        <div className="lg-spells__stat" data-testid="sheet-spell-save-dc">
          <span className="lg-spells__label">{saveLabel}</span>
          <span className="lg-spells__number">{numberOrMissing(saveDC)}</span>
        </div>
      )}
    </>
  );
}

function SpellText({ path }: { readonly path: string }) {
  const field = usePresetField(path);
  const label = usePresetLabel(path);
  const value = usePresetValue(path);
  const commit = usePresetCommit(path);

  if (field === undefined) {
    return null;
  }

  return (
    <div className="lg-spells__text" data-testid={`sheet-spell-${lastSegment(path)}`}>
      <span className="lg-spells__label">{label}</span>
      <InlineEdit
        className="lg-spells__text-value lg-inline-edit--quiet"
        label={label}
        path={path}
        schema={field.schema}
        value={textValue(value)}
        onCommit={commit}
      />
    </div>
  );
}

interface SlotLevelProps {
  readonly level: number;
  readonly pool: SlotPool;
  readonly totalValue: unknown;
  readonly totalSchema: SheetField;
  readonly onSpend: () => void;
  readonly onRecover: () => void;
  readonly onTotal: (value: unknown) => void;
}

function SlotLevel({
  level,
  pool,
  totalValue,
  totalSchema,
  onSpend,
  onRecover,
  onTotal,
}: SlotLevelProps) {
  const groupLabel = messageWith('sheet.play.slotGroup', { count: level });
  const totalNumber = numberValue(totalValue);

  return (
    <li className="lg-spells__slot" data-testid={`sheet-slot-${level}`}>
      <span className="lg-spells__slot-level">
        {messageWith('sheet.play.level', { count: level })}
      </span>
      <div className="lg-spells__pips" role="group" aria-label={groupLabel}>
        {slotPips(pool).map((pip) => (
          <button
            key={pip.index}
            type="button"
            className="lg-spells__pip"
            data-edit-path={slotPath(level, 'expended')}
            data-spent={pip.spent ? '' : undefined}
            aria-label={
              pip.spent
                ? messageWith('sheet.play.slotRecover', { count: level })
                : messageWith('sheet.play.slotSpend', { count: level })
            }
            onClick={pip.spent ? onRecover : onSpend}
          />
        ))}
      </div>
      <span className="lg-spells__slot-count">
        <span className="lg-spells__slot-remaining" data-testid={`sheet-slot-${level}-remaining`}>
          <span className="lg-visually-hidden">
            {messageWith('sheet.play.slotsLeft', {
              remaining: pool.remaining,
              total: pool.total,
            })}
          </span>
          <span aria-hidden="true">{pool.remaining}</span>
        </span>
        <span aria-hidden="true">/</span>
        <InlineEdit
          className="lg-spells__slot-total"
          label={totalSchema.schema.label ?? message('sheet.play.slotTotal')}
          path={totalSchema.path}
          schema={totalSchema.schema}
          value={totalNumber === undefined ? '' : String(totalNumber)}
          inputMode="numeric"
          onCommit={(text) => {
            const next = parseClampedInt(text, 0, UNBOUNDED);
            if (next !== undefined) {
              onTotal(next);
            }
          }}
        />
      </span>
    </li>
  );
}

function SpellSlots() {
  const { store, fields } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const { rows, hidden } = useMemo(() => {
    const drawn: { level: number; field: SheetField; pool: SlotPool; totalValue: unknown }[] = [];
    const hiddenLevels: number[] = [];
    for (const level of SLOT_LEVELS) {
      const totalField = fields.get(slotPath(level, 'total'));
      if (totalField === undefined) {
        continue;
      }
      const totalValue = valueAtPath(state.data, totalField.path);
      const pooled = slotPool(totalValue, valueAtPath(state.data, slotPath(level, 'expended')));
      const edited = state.editedPaths.has(totalField.path);
      if (pooled.total === 0 && !edited) {
        hiddenLevels.push(level);
      } else {
        drawn.push({ level, field: totalField, pool: pooled, totalValue });
      }
    }
    return { rows: drawn, hidden: hiddenLevels };
  }, [fields, state.data, state.editedPaths]);

  if (rows.length === 0 && hidden.length === 0) {
    return null;
  }

  return (
    <div className="lg-spells__slots" data-testid="sheet-spell-slots">
      {rows.length > 0 && (
        <>
          <h3 className="lg-spells__subtitle">{message('sheet.play.spellSlots')}</h3>
          <ul className="lg-spells__slot-list">
            {rows.map(({ level, field, pool, totalValue }) => (
              <SlotLevel
                key={level}
                level={level}
                pool={pool}
                totalValue={totalValue}
                totalSchema={field}
                onSpend={() => store.setPath(slotPath(level, 'expended'), spendSlot(pool).expended)}
                onRecover={() =>
                  store.setPath(slotPath(level, 'expended'), recoverSlot(pool).expended)
                }
                onTotal={(value) => store.setPath(slotPath(level, 'total'), value)}
              />
            ))}
          </ul>
        </>
      )}
      {hidden.length > 0 && (
        <p className="lg-spells__hidden-levels" data-testid="sheet-spell-hidden-levels">
          {hidden.length === 1
            ? messageWith('sheet.play.hiddenLevel', { levels: formatLevelRanges(hidden) })
            : messageWith('sheet.play.hiddenLevels', { levels: formatLevelRanges(hidden) })}
        </p>
      )}
    </div>
  );
}

export function FantasySpells() {
  const { fields } = usePresetContext();
  const titleId = useId();
  const hasSpells =
    fields.has(SPELL_ATTACK_PATH) ||
    fields.has(SPELL_SAVE_DC_PATH) ||
    fields.has('spells.atWill') ||
    fields.has('spells.list') ||
    fields.has(slotPath(1, 'total'));

  if (!hasSpells) {
    return null;
  }

  return (
    <section
      className="lg-play-section lg-spells"
      data-testid="sheet-spells"
      aria-labelledby={titleId}
    >
      <h2 id={titleId} className="lg-play-section__title">
        {message('sheet.play.spellcasting')}
      </h2>
      <div className="lg-spells__top">
        <SpellcastingAbility />
        <SpellStats />
      </div>
      <SpellSlots />
      <SpellText path="spells.atWill" />
      <SpellText path="spells.list" />
    </section>
  );
}
