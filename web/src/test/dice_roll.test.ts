import { describe, expect, it, vi } from 'vitest';
import {
  dieFromWords,
  evaluateExpression,
  parseDice,
  performRoll,
  resolveDice,
  rollExpression,
  schemaFromKinds,
  type DiceChoices,
  type DiceDefaults,
  type Expression,
  type RandomWords,
  type Roller,
} from '../dice';
import { diceVectors, nestValues } from './dice_vectors';

const MAX_WORD = 0xffffffff;

function expression(expr: string): Expression {
  const parsed = parseDice(expr);
  if (!parsed.ok) {
    throw new Error(`fixture does not parse: ${expr}`);
  }
  return parsed.value;
}

/** A roller that replays fixed results and records how many it handed out. */
function fixedRoller(results: readonly number[]): Roller & { readonly used: () => number } {
  let next = 0;
  const roller: Roller = () => {
    const value = results[next];
    next += 1;
    return value;
  };
  return Object.assign(roller, { used: () => next });
}

function wordsFrom(sequence: readonly number[]): RandomWords & { readonly calls: () => number } {
  let next = 0;
  const source: RandomWords = () => {
    const value = sequence[next % sequence.length];
    next += 1;
    return value;
  };
  return Object.assign(source, { calls: () => next });
}

describe('evaluateExpression against the shared vectors', () => {
  it.each(diceVectors.evaluate)('Vector: $name', (vector) => {
    const roller = fixedRoller(vector.dice);
    const result = evaluateExpression(expression(vector.expr), {
      rowKey: vector.row,
      values: nestValues(vector.values),
      roller,
    });
    if (vector.error !== undefined) {
      expect(!result.ok && result.error.class).toBe(vector.error);
      return;
    }
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.total).toBe(vector.total);
      expect(result.value.parts).toEqual(vector.parts);
      expect(roller.used()).toBe(vector.dice.length);
    }
  });
});

describe('what a Roll reads', () => {
  const read = (expr: string, values: Record<string, unknown>, draft?: Record<string, unknown>) =>
    evaluateExpression(expression(expr), {
      rowKey: 'combat.initiative',
      values,
      draft,
      roller: fixedRoller([]),
    });

  it.each([
    ['missing', {}],
    ['null', { combat: { initiative: null } }],
    ['NaN', { combat: { initiative: Number.NaN } }],
    ['Infinity', { combat: { initiative: Number.POSITIVE_INFINITY } }],
    ['-Infinity', { combat: { initiative: Number.NEGATIVE_INFINITY } }],
    ['a numeric string', { combat: { initiative: '5' } }],
    ['text', { combat: { initiative: 'five' } }],
    ['a boolean', { combat: { initiative: true } }],
    ['an object', { combat: { initiative: {} } }],
    ['an array', { combat: { initiative: [3] } }],
    ['a parent that is not an object', { combat: 4 }],
    ['only inherited', { combat: Object.create({ initiative: 5 }) as object }],
  ])('Error: a value that is %s gives no_value, never a silent 0', (_label, values) => {
    const result = read('@', values);
    expect(!result.ok && result.error.class).toBe('no_value');
    expect(!result.ok && result.error.message).toContain('combat.initiative');
  });

  it('Error: the error names the first missing reference even when other terms are fine', () => {
    const result = read('stats.strength + @', { stats: { strength: 10 } });
    expect(!result.ok && result.error.message).toContain('combat.initiative');
  });

  it.each([
    [2.7, 2],
    [-2.7, -3],
    [-0.2, -1],
    [0.9999, 0],
    [7, 7],
    [-0, 0],
    [-4, -4],
  ])('Edge: %d is floored toward minus infinity to %d', (raw, floored) => {
    const result = read('@', { combat: { initiative: raw } });
    expect(result.ok && Object.is(result.value.total, floored)).toBe(true);
    expect(result.ok && Object.is(result.value.parts[0].value, floored)).toBe(true);
  });

  it('Edge: a subtracted negative zero never leaks a -0 total', () => {
    const result = read('-@', { combat: { initiative: 0 } });
    expect(result.ok && Object.is(result.value.total, 0)).toBe(true);
  });

  it('Edge: the unsaved draft wins over the saved value', () => {
    const result = read('@', { combat: { initiative: 2 } }, { combat: { initiative: 9 } });
    expect(result.ok && result.value.total).toBe(9);
  });

  it('Edge: a path the draft does not hold falls back to the saved value', () => {
    const result = read(
      'combat.initiative + stats.strength',
      { combat: { initiative: 2 }, stats: { strength: 10 } },
      { combat: { initiative: 3 } },
    );
    expect(result.ok && result.value.total).toBe(13);
  });

  it('Error: a draft that cleared the field is read as empty, not as the stale saved number', () => {
    const result = read('@', { combat: { initiative: 2 } }, { combat: { initiative: null } });
    expect(!result.ok && result.error.class).toBe('no_value');
  });

  it('Edge: an empty draft changes nothing', () => {
    const result = read('@', { combat: { initiative: 2 } }, {});
    expect(result.ok && result.value.total).toBe(2);
  });

  it('Error: a total that overflows is refused rather than returned as Infinity', () => {
    const result = read('combat.initiative + stats.strength', {
      combat: { initiative: Number.MAX_VALUE },
      stats: { strength: Number.MAX_VALUE },
    });
    expect(!result.ok && result.error.class).toBe('overflow');
  });

  it('Edge: a Roll never mutates the values it reads', () => {
    const values = Object.freeze({ combat: Object.freeze({ initiative: 2.5 }) });
    expect(read('@', values).ok).toBe(true);
    expect(values.combat.initiative).toBe(2.5);
  });
});

describe('dieFromWords', () => {
  const faceCounts = [1, 2, 3, 4, 6, 8, 10, 12, 20, 100, 999, 1000];
  const extremes = [0, 1, 2, 5, 6, 0x7fffffff, 0x80000000, MAX_WORD - 1, MAX_WORD];

  it.each(faceCounts)('Edge: 1d%d stays within 1..%d for every extreme of the source', (faces) => {
    for (const word of extremes) {
      const result = dieFromWords(faces, wordsFrom([word, 0]));
      expect(result).toBeGreaterThanOrEqual(1);
      expect(result).toBeLessThanOrEqual(faces);
      expect(Number.isInteger(result)).toBe(true);
    }
  });

  it('Edge: the lowest word is face 1 and the last accepted word of a power-of-two die is its top face', () => {
    expect(dieFromWords(20, wordsFrom([0]))).toBe(1);
    expect(dieFromWords(2, wordsFrom([MAX_WORD]))).toBe(2);
    expect(dieFromWords(1, wordsFrom([MAX_WORD]))).toBe(1);
    expect(dieFromWords(1024, wordsFrom([MAX_WORD]))).toBe(1024);
  });

  it('Edge: no modulo bias: for 1d6 the words below 4294967292 are used and the 4 above it are redrawn', () => {
    expect(dieFromWords(6, wordsFrom([4294967291]))).toBe(6);
    const redrawn = wordsFrom([4294967292, 4294967293, 4294967294, MAX_WORD, 7]);
    expect(dieFromWords(6, redrawn)).toBe(2);
    expect(redrawn.calls()).toBe(5);
  });

  it('Edge: consecutive words walk the faces evenly', () => {
    const seen = new Array<number>(6).fill(0);
    for (let word = 0; word < 60; word += 1) {
      seen[dieFromWords(6, wordsFrom([word])) - 1] += 1;
    }
    expect(seen).toEqual([10, 10, 10, 10, 10, 10]);
  });

  it('Error: a source stuck in the rejected range fails loudly instead of looping forever', () => {
    expect(() => dieFromWords(6, wordsFrom([MAX_WORD]))).toThrow(/random source/);
  });
});

describe('rollExpression', () => {
  const values = { combat: { initiative: 2 } };

  it('Happy path: a Roll returns the total and the breakdown per die and per number', () => {
    const result = rollExpression(expression('2d6 + @ - 1'), {
      rowKey: 'combat.initiative',
      values,
      words: wordsFrom([2, 4]),
    });
    expect(result.ok && result.value).toEqual({
      total: 9,
      parts: [
        { sign: 1, label: '2d6', value: 8, rolls: [3, 5] },
        { sign: 1, label: 'combat.initiative', value: 2 },
        { sign: -1, label: '1', value: 1 },
      ],
    });
  });

  it('Edge: every random value yields 1..M on 1dM, including both ends of the source', () => {
    for (const faces of [1, 6, 20, 1000]) {
      for (const word of [0, MAX_WORD - 1, MAX_WORD]) {
        const result = rollExpression(expression(`1d${faces}`), {
          rowKey: 'x',
          values: {},
          words: wordsFrom([word, 0]),
        });
        const roll = result.ok ? result.value.total : Number.NaN;
        expect(roll).toBeGreaterThanOrEqual(1);
        expect(roll).toBeLessThanOrEqual(faces);
      }
    }
  });

  it('Edge: the largest expression draws one word per die', () => {
    const source = wordsFrom([0]);
    const result = rollExpression(expression('20d1000 + 20d1000'), {
      rowKey: 'x',
      values: {},
      words: source,
    });
    expect(result.ok && result.value.total).toBe(40);
    expect(source.calls()).toBe(40);
  });

  it('Edge: the default source is crypto.getRandomValues and rolls stay in range', () => {
    const spy = vi.spyOn(globalThis.crypto, 'getRandomValues');
    for (let attempt = 0; attempt < 200; attempt += 1) {
      const result = rollExpression(expression('1d20'), { rowKey: 'x', values: {} });
      const total = result.ok ? result.value.total : Number.NaN;
      expect(total).toBeGreaterThanOrEqual(1);
      expect(total).toBeLessThanOrEqual(20);
    }
    expect(spy).toHaveBeenCalled();
    spy.mockRestore();
  });

  it('Error: a missing value fails the Roll and reports nothing partial', () => {
    const result = rollExpression(expression('1d20 + @'), {
      rowKey: 'combat.initiative',
      values: {},
      words: wordsFrom([0]),
    });
    expect(result.ok).toBe(false);
    expect('value' in result).toBe(false);
  });
});

describe('performRoll (resolution and Roll together)', () => {
  const schema = schemaFromKinds(diceVectors.fields);
  const template: DiceDefaults = {
    sectionDice: { skills: '1d20 + @.total' },
    rowDice: { 'combat.initiative': '1d20 + @' },
  };
  const sheet = {
    skills: { athletics: { bonus: 0, total: 3 } },
    combat: { initiative: 2, hitPoints: 10 },
  };

  function rollRow(rowKey: string, sectionKey: string, choices: DiceChoices) {
    const resolution = resolveDice({ rowKey, sectionKey, schema, template, choices });
    return performRoll(resolution, { rowKey, values: sheet, words: wordsFrom([0]) });
  }

  it('Error: a Row with no dice is refused with the resolution reason', () => {
    const result = rollRow('combat.hitPoints', 'combat', {});
    expect(!result.ok && result.error.class).toBe('no_dice');
  });

  it('Error: a Row turned off is refused with the resolution reason', () => {
    const result = rollRow('combat.initiative', 'combat', { row: { 'combat.initiative': null } });
    expect(!result.ok && result.error.class).toBe('turned_off');
  });

  it('Error: a Row whose expression does not fit is refused, not rolled with a guess', () => {
    const result = rollRow('combat.initiative', 'combat', {
      row: { 'combat.initiative': '1d20 + @.total' },
    });
    expect(!result.ok && result.error.class).toBe('unknown_field');
  });

  it('Happy path: the Template default rolls and breaks down per die and per number', () => {
    const result = rollRow('combat.initiative', 'combat', {});
    expect(result.ok && result.value).toEqual({
      total: 3,
      parts: [
        { sign: 1, label: '1d20', value: 1, rolls: [1] },
        { sign: 1, label: 'combat.initiative', value: 2 },
      ],
    });
  });

  it('Happy path: the player flat bonus is added on top of the Section default', () => {
    const result = rollRow('skills.athletics', 'skills', {
      row: { 'skills.athletics': '1d20 + @.total + 2' },
    });
    expect(result.ok && result.value.total).toBe(1 + 3 + 2);
    expect(result.ok && result.value.parts.map((part) => part.label)).toEqual([
      '1d20',
      'skills.athletics.total',
      '2',
    ]);
  });
});
