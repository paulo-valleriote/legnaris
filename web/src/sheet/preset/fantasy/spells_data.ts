import { valueAtPath } from '../../../api/models/sheet';
import type { SheetField } from '../../../engine/field_grouping';
import { textValue } from '../values';
import { slotPool } from './slot_pips';

export const SPELLCASTING_ABILITY_GROUP = 'spellcastingAbility';

export const SPELL_ATTACK_PATH = 'spells.attackBonus';

export const SPELL_SAVE_DC_PATH = 'spells.saveDC';

export const AT_WILL_PATH = 'spells.atWill';

export const LIST_PATH = 'spells.list';

export const SPELL_ATTACK_DICE = '1d20 + @';

export const SLOT_LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

export function slotPath(level: number, leaf: 'total' | 'expended'): string {
  return `spells.slot${level}.${leaf}`;
}

export interface SpellCastInput {
  readonly data: Record<string, unknown>;
  readonly fields: ReadonlyMap<string, SheetField>;
  readonly editedPaths: ReadonlySet<string>;
}

export function hasSpellCast({ data, fields, editedPaths }: SpellCastInput): boolean {
  if (textValue(valueAtPath(data, AT_WILL_PATH)).trim() !== '') {
    return true;
  }
  if (textValue(valueAtPath(data, LIST_PATH)).trim() !== '') {
    return true;
  }
  return SLOT_LEVELS.some((level) => {
    const totalField = fields.get(slotPath(level, 'total'));
    if (totalField === undefined) {
      return false;
    }
    const pool = slotPool(
      valueAtPath(data, totalField.path),
      valueAtPath(data, slotPath(level, 'expended')),
    );
    return pool.total > 0 || editedPaths.has(totalField.path);
  });
}
