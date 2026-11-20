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
import {
  AT_WILL_PATH,
  hasSpellCast,
  LIST_PATH,
  SLOT_LEVELS,
  SPELL_ATTACK_DICE,
  SPELL_ATTACK_PATH,
  SPELL_SAVE_DC_PATH,
  SPELLCASTING_ABILITY_GROUP,
  slotPath,
} from './spells_data';
import './play_section.css';
import './spells.css';

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

function SpellProseLine({ path }: { readonly path: string }) {
  const field = usePresetField(path);
  const label = usePresetLabel(path);
  const value = usePresetValue(path);
  const commit = usePresetCommit(path);
  const text = textValue(value);

  if (field === undefined || text.trim() === '') {
    return null;
  }

  return (
    <p className="lg-spells__prose-line" data-testid={`sheet-spell-${lastSegment(path)}`}>
      <span className="lg-spells__prose-term">{label}</span>{' '}
      <InlineEdit
        className="lg-spells__prose-value lg-inline-edit--quiet"
        label={label}
        path={path}
        schema={field.schema}
        value={text}
        onCommit={commit}
      />
    </p>
  );
}

function SpellProse() {
  const atWill = textValue(usePresetValue(AT_WILL_PATH));
  const list = textValue(usePresetValue(LIST_PATH));

  if (atWill.trim() === '' && list.trim() === '') {
    return null;
  }

  return (
    <div className="lg-spells__prose" data-testid="sheet-spell-prose">
      <SpellProseLine path={AT_WILL_PATH} />
      <SpellProseLine path={LIST_PATH} />
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

export function SpellSlots({ compact = false }: { readonly compact?: boolean }) {
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

  const hasRows = rows.length > 0;
  const showHidden = hidden.length > 0 && (!compact || hasRows);

  if (!hasRows && !showHidden) {
    return null;
  }

  return (
    <div className="lg-spells__slots" data-testid="sheet-spell-slots">
      {hasRows && (
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
      {showHidden && (
        <p className="lg-spells__hidden-levels" data-testid="sheet-spell-hidden-levels">
          {hidden.length === 1
            ? messageWith('sheet.play.hiddenLevel', { levels: formatLevelRanges(hidden) })
            : messageWith('sheet.play.hiddenLevels', { levels: formatLevelRanges(hidden) })}
        </p>
      )}
    </div>
  );
}

function hasSpellFields(fields: ReadonlyMap<string, SheetField>): boolean {
  return (
    fields.has(SPELL_ATTACK_PATH) ||
    fields.has(SPELL_SAVE_DC_PATH) ||
    fields.has(AT_WILL_PATH) ||
    fields.has(LIST_PATH) ||
    fields.has(slotPath(1, 'total'))
  );
}

function SpellcastSection({
  compact,
  titleId,
}: {
  readonly compact: boolean;
  readonly titleId: string;
}) {
  const variant = compact ? 'compact' : 'panel';

  return (
    <section
      className={
        compact ? 'lg-play-section lg-spells lg-spells--compact' : 'lg-play-section lg-spells'
      }
      data-testid="sheet-spells"
      data-variant={variant}
      aria-labelledby={titleId}
    >
      <h2 id={titleId} className="lg-play-section__title">
        {message('sheet.play.spellcasting')}
      </h2>
      <div className="lg-spells__top" data-testid={compact ? 'sheet-spell-summary' : undefined}>
        <SpellcastingAbility />
        <SpellStats />
      </div>
      <SpellSlots compact={compact} />
      {compact ? (
        <SpellProse />
      ) : (
        <>
          <SpellText path={AT_WILL_PATH} />
          <SpellText path={LIST_PATH} />
        </>
      )}
    </section>
  );
}

function PanelSpellcast({ titleId }: { readonly titleId: string }) {
  const { fields } = usePresetContext();

  if (!hasSpellFields(fields)) {
    return null;
  }

  return <SpellcastSection compact={false} titleId={titleId} />;
}

function CompactSpellcast({ titleId }: { readonly titleId: string }) {
  const { fields, store } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);

  if (
    !hasSpellFields(fields) ||
    !hasSpellCast({ data: state.data, fields, editedPaths: state.editedPaths })
  ) {
    return null;
  }

  return <SpellcastSection compact titleId={titleId} />;
}

export type SpellsVariant = 'panel' | 'compact';

interface FantasySpellsProps {
  readonly variant?: SpellsVariant;
}

export function FantasySpells({ variant = 'panel' }: FantasySpellsProps) {
  const titleId = useId();

  return variant === 'compact' ? (
    <CompactSpellcast titleId={titleId} />
  ) : (
    <PanelSpellcast titleId={titleId} />
  );
}
