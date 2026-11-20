import { parseDice, type Expression } from '../../../dice/parse';
import { parseClampedInt } from './parse_number';

export const ATTACK_SLOT_COUNT = 4;

export const ATTACK_BONUS_MIN = -5;

export const ATTACK_BONUS_MAX = 20;

export interface AttackSlot {
  readonly slot: number;
  readonly rowKey: string;
  readonly namePath: string;
  readonly damagePath: string;
  readonly bonusPath: string;
}

export function attackSlot(slot: number): AttackSlot {
  const rowKey = `attacks.attack${slot}`;
  return {
    slot,
    rowKey,
    namePath: `${rowKey}.name`,
    damagePath: `${rowKey}.damage`,
    bonusPath: `${rowKey}.bonus`,
  };
}

export const ATTACK_SLOTS: readonly AttackSlot[] = Array.from(
  { length: ATTACK_SLOT_COUNT },
  (_, index) => attackSlot(index + 1),
);

export function attackFilled(name: unknown): boolean {
  return typeof name === 'string' && name.trim() !== '';
}

export function parseAttackBonus(text: string): number | undefined {
  return parseClampedInt(text, ATTACK_BONUS_MIN, ATTACK_BONUS_MAX);
}

export function parseAttackDamage(text: string): Expression | undefined {
  const parsed = parseDice(text.trim());
  if (!parsed.ok) {
    return undefined;
  }
  const rollsDice = parsed.value.terms.some((term) => term.kind === 'dice');
  const readsField = parsed.value.terms.some((term) => term.kind === 'field');
  return rollsDice && !readsField ? parsed.value : undefined;
}
