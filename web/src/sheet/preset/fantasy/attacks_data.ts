import { parseDice, type Expression } from '../../../dice/parse';

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

export interface AttackVisibility {
  readonly shown: readonly number[];
  readonly free: readonly number[];
  readonly next: number | undefined;
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

export function attackVisibility(
  names: readonly unknown[],
  revealed: readonly number[],
): AttackVisibility {
  const shown: number[] = [];
  const free: number[] = [];
  for (let index = 0; index < names.length; index += 1) {
    const slot = index + 1;
    const filled = attackFilled(names[index]);
    const isRevealed = revealed.includes(slot);
    if (filled || isRevealed) {
      shown.push(slot);
    }
    if (!filled && !isRevealed) {
      free.push(slot);
    }
  }
  return { shown, free, next: free[0] };
}

export function parseAttackBonus(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  const rounded = Math.round(parsed);
  return Math.min(ATTACK_BONUS_MAX, Math.max(ATTACK_BONUS_MIN, rounded));
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
