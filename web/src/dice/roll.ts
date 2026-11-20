import { refuse, succeed, type DiceResult } from './errors';
import { resolveFieldPath } from './fit';
import { termLabel, type Expression, type Term } from './parse';
import type { Resolution } from './resolve';

export type Roller = (faces: number) => number;

export type RandomWords = () => number;

export interface RollPart {
  readonly sign: 1 | -1;
  readonly label: string;
  readonly value: number;
  readonly rolls?: readonly number[];
  readonly discarded?: readonly number[];
}

export type DiceKeep = 'highest' | 'lowest';

export type RollMode = 'normal' | 'advantage' | 'disadvantage';

export const D20_FACES = 20;

export function keepForMode(mode: RollMode): DiceKeep | undefined {
  switch (mode) {
    case 'advantage':
      return 'highest';
    case 'disadvantage':
      return 'lowest';
    default:
      return undefined;
  }
}

export function rollsD20(expression: Expression): boolean {
  return expression.terms.some((term) => term.kind === 'dice' && term.faces === D20_FACES);
}

export function keptIndices(
  rolls: readonly number[],
  count: number,
  keep: DiceKeep,
): readonly number[] {
  const order = rolls.map((roll, index) => ({ roll, index }));
  order.sort((left, right) => {
    const byValue = keep === 'highest' ? right.roll - left.roll : left.roll - right.roll;
    return byValue === 0 ? left.index - right.index : byValue;
  });
  return order
    .slice(0, count)
    .map((entry) => entry.index)
    .sort((left, right) => left - right);
}

export interface RollResult {
  readonly total: number;
  readonly parts: readonly RollPart[];
}

type Tree = Readonly<Record<string, unknown>>;

export interface EvaluateContext {
  readonly rowKey: string;
  readonly values: Tree;
  readonly draft?: Tree;
  readonly roller: Roller;
  readonly keep?: DiceKeep;
}

export interface RollContext extends Omit<EvaluateContext, 'roller'> {
  readonly words?: RandomWords;
}

const WORD_SPACE = 2 ** 32;
const MAX_REDRAWS = 16;

export function cryptoWords(): number {
  return globalThis.crypto.getRandomValues(new Uint32Array(1))[0];
}

export function dieFromWords(faces: number, words: RandomWords): number {
  const limit = WORD_SPACE - (WORD_SPACE % faces);
  for (let attempt = 0; attempt < MAX_REDRAWS; attempt += 1) {
    const word = words();
    if (word < limit) {
      return (word % faces) + 1;
    }
  }
  throw new Error('the random source keeps returning words outside the die range');
}

type Lookup = { readonly found: true; readonly value: unknown } | { readonly found: false };

function lookup(tree: Tree | undefined, path: string): Lookup {
  let cursor: unknown = tree;
  for (const segment of path.split('.')) {
    if (typeof cursor !== 'object' || cursor === null || !Object.hasOwn(cursor, segment)) {
      return { found: false };
    }
    cursor = (cursor as Record<string, unknown>)[segment];
  }
  return { found: true, value: cursor };
}

function readNumber(context: EvaluateContext, path: string): number | undefined {
  const drafted = lookup(context.draft, path);
  const found = drafted.found ? drafted : lookup(context.values, path);
  const value = found.found ? found.value : undefined;
  return typeof value === 'number' && Number.isFinite(value) ? Math.floor(value) : undefined;
}

const withoutNegativeZero = (value: number): number => (value === 0 ? 0 : value);

function evaluateTerm(term: Term, context: EvaluateContext): DiceResult<RollPart> {
  const { sign } = term;
  switch (term.kind) {
    case 'dice': {
      const keep = term.faces === D20_FACES ? context.keep : undefined;
      const throws = term.count + (keep === undefined ? 0 : 1);
      const rolls = Array.from({ length: throws }, () => context.roller(term.faces));
      if (keep === undefined) {
        const value = rolls.reduce((sum, roll) => sum + roll, 0);
        return succeed({ sign, label: termLabel(term), value, rolls });
      }
      const kept = keptIndices(rolls, term.count, keep);
      const value = kept.reduce((sum, index) => sum + rolls[index], 0);
      const discarded = rolls.map((_, index) => index).filter((index) => !kept.includes(index));
      return succeed({ sign, label: termLabel(term), value, rolls, discarded });
    }
    case 'number':
      return succeed({ sign, label: termLabel(term), value: term.value });
    case 'field': {
      const path = resolveFieldPath(term, context.rowKey);
      const value = readNumber(context, path);
      if (value === undefined) {
        return refuse('no_value', `"${path}" has no number to read`);
      }
      return succeed({ sign, label: path, value: withoutNegativeZero(value) });
    }
  }
}

export function evaluateExpression(
  expression: Expression,
  context: EvaluateContext,
): DiceResult<RollResult> {
  const parts: RollPart[] = [];
  let total = 0;
  for (const term of expression.terms) {
    const part = evaluateTerm(term, context);
    if (!part.ok) {
      return part;
    }
    total += part.value.sign * part.value.value;
    if (!Number.isFinite(total)) {
      return refuse('overflow', 'the total is too large to compute');
    }
    parts.push(part.value);
  }
  return succeed({ total: withoutNegativeZero(total), parts });
}

export function rollExpression(
  expression: Expression,
  context: RollContext,
): DiceResult<RollResult> {
  const words = context.words ?? cryptoWords;
  return evaluateExpression(expression, {
    rowKey: context.rowKey,
    values: context.values,
    draft: context.draft,
    keep: context.keep,
    roller: (faces) => dieFromWords(faces, words),
  });
}

export function performRoll(resolution: Resolution, context: RollContext): DiceResult<RollResult> {
  if (!resolution.rollable) {
    return { ok: false, error: resolution.reason };
  }
  return rollExpression(resolution.parsed, context);
}
