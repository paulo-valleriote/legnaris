import type { DiceResult } from '../dice/errors';
import type { RollMode, RollResult } from '../dice/roll';

export const ROLL_HISTORY_LIMIT = 3;

export interface RollEntry {
  readonly id: number;
  readonly label: string;
  readonly outcome: DiceResult<RollResult>;
  readonly mode?: RollMode;
}

export function rememberRoll(
  history: readonly RollEntry[],
  entry: RollEntry,
): readonly RollEntry[] {
  return [entry, ...history].slice(0, ROLL_HISTORY_LIMIT);
}
