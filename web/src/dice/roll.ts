/**
 * Performing a Roll (ADR 0005): throw each die, read the Fields the expression
 * names at that moment, floor them, and return the total with every die and
 * number that made it. Ephemeral: nothing is stored, there is no history.
 * Mirrors `Expression.Evaluate` of `backend/internal/dice/evaluate.go`; the
 * shared vectors pin both.
 */

import { refuse, succeed, type DiceResult } from './errors';
import { resolveFieldPath } from './fit';
import { termLabel, type Expression, type Term } from './parse';
import type { Resolution } from './resolve';

/** Returns one die result in `[1, faces]`. */
export type Roller = (faces: number) => number;

/** Returns an unsigned 32-bit random word, `0 … 2^32 - 1`. */
export type RandomWords = () => number;

/** One term of a Roll: what it was, its sign, and the value it contributed before the sign applies. */
export interface RollPart {
  readonly sign: 1 | -1;
  readonly label: string;
  readonly value: number;
  /** Every die of a dice term, in the order they were thrown; absent on a number or a Field. */
  readonly rolls?: readonly number[];
}

export interface RollResult {
  readonly total: number;
  readonly parts: readonly RollPart[];
}

/** A value tree as the sheet holds it: nested objects keyed by path segment. */
type Tree = Readonly<Record<string, unknown>>;

export interface EvaluateContext {
  /** Key of the Row being rolled; `@` resolves against it. */
  readonly rowKey: string;
  /** The values on screen (saved ones). */
  readonly values: Tree;
  /** Unsaved edits: a path present here wins over `values`, even when it holds an empty value. */
  readonly draft?: Tree;
  readonly roller: Roller;
}

export interface RollContext extends Omit<EvaluateContext, 'roller'> {
  /** Defaults to `crypto.getRandomValues`. */
  readonly words?: RandomWords;
}

const WORD_SPACE = 2 ** 32;
const MAX_REDRAWS = 16;

/** The system CSPRNG, one 32-bit word at a time. */
export function cryptoWords(): number {
  return globalThis.crypto.getRandomValues(new Uint32Array(1))[0];
}

/**
 * Throws one die of [faces] faces from [words] without modulo bias: a word at or
 * above the largest multiple of [faces] that fits in 32 bits is redrawn. The
 * redraw is capped because a source that only returns rejected words is a broken
 * source, not bad luck (the odds for a real one are below 2^-32 per draw).
 */
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

/** `-0` would print and compare as a different number than `0`. */
const withoutNegativeZero = (value: number): number => (value === 0 ? 0 : value);

function evaluateTerm(term: Term, context: EvaluateContext): DiceResult<RollPart> {
  const { sign } = term;
  switch (term.kind) {
    case 'dice': {
      const rolls = Array.from({ length: term.count }, () => context.roller(term.faces));
      const value = rolls.reduce((sum, roll) => sum + roll, 0);
      return succeed({ sign, label: termLabel(term), value, rolls });
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

/**
 * Evaluates [expression] for the Row in [context]: dice are thrown in expression
 * order through the roller, a Field is read (draft first), floored toward minus
 * infinity and added. A Field that is missing, `null`, `NaN`, infinite or not a
 * number gives `no_value`, never a silent 0; a total that grows to infinity gives `overflow`. It does not check the schema; use
 * `fitsRow` / `resolveDice` for that.
 */
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

/** Rolls [expression] with a local random source (the system CSPRNG unless [RollContext.words] says otherwise). */
export function rollExpression(
  expression: Expression,
  context: RollContext,
): DiceResult<RollResult> {
  const words = context.words ?? cryptoWords;
  return evaluateExpression(expression, {
    rowKey: context.rowKey,
    values: context.values,
    draft: context.draft,
    roller: (faces) => dieFromWords(faces, words),
  });
}

/** Rolls the expression a {@link Resolution} chose; a Row that cannot be rolled is refused with the resolution's reason. */
export function performRoll(resolution: Resolution, context: RollContext): DiceResult<RollResult> {
  if (!resolution.rollable) {
    return { ok: false, error: resolution.reason };
  }
  return rollExpression(resolution.parsed, context);
}
