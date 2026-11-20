import { describe, expect, it } from 'vitest';
import {
  keepForMode,
  keptIndices,
  parseDice,
  performRoll,
  resolveDice,
  rollExpression,
  rollsD20,
  schemaFromKinds,
  type DiceChoices,
  type Expression,
  type RandomWords,
} from '../dice';

function expression(expr: string): Expression {
  const parsed = parseDice(expr);
  if (!parsed.ok) {
    throw new Error(`fixture does not parse: ${expr}`);
  }
  return parsed.value;
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

const wordsFor = (faces: readonly number[]) => wordsFrom(faces.map((face) => face - 1));

describe('rollsD20', () => {
  it.each([
    ['1d20', true],
    ['d20', true],
    ['1d20 + 3', true],
    ['2d20 - 1', true],
    ['1d8 + 3', false],
    ['2d6', false],
    ['1d100 + 1d12', false],
    ['@', false],
    ['3', false],
  ])('Edge: %s is a d20 expression: %s', (expr, expected) => {
    expect(rollsD20(expression(expr))).toBe(expected);
  });
});

describe('keepForMode', () => {
  it.each([
    ['normal', undefined],
    ['advantage', 'highest'],
    ['disadvantage', 'lowest'],
  ] as const)('Edge: %s keeps %s', (mode, expected) => {
    expect(keepForMode(mode)).toBe(expected);
  });
});

describe('keptIndices', () => {
  it('Happy path: keeping the highest of two dice names the second one', () => {
    expect(keptIndices([4, 17], 1, 'highest')).toEqual([1]);
  });

  it('Happy path: keeping the lowest of two dice names the first one', () => {
    expect(keptIndices([4, 17], 1, 'lowest')).toEqual([0]);
  });

  it('Edge: equal dice keep the earliest throw, so the mark is stable', () => {
    expect(keptIndices([9, 9], 1, 'highest')).toEqual([0]);
    expect(keptIndices([9, 9], 1, 'lowest')).toEqual([0]);
  });

  it('Edge: keeping several dice returns their indices in throw order', () => {
    expect(keptIndices([2, 20, 10], 2, 'highest')).toEqual([1, 2]);
    expect(keptIndices([2, 20, 10], 2, 'lowest')).toEqual([0, 2]);
  });
});

describe('rolling a d20 with a keep mode', () => {
  it('Happy path: advantage throws two dice, keeps the higher one and adds the modifier', () => {
    const result = rollExpression(expression('1d20 + 3'), {
      rowKey: 'combat.initiative',
      values: {},
      words: wordsFor([5, 17]),
      keep: 'highest',
    });

    expect(result.ok && result.value).toEqual({
      total: 20,
      parts: [
        { sign: 1, label: '1d20', value: 17, rolls: [5, 17], discarded: [0] },
        { sign: 1, label: '3', value: 3 },
      ],
    });
  });

  it('Happy path: disadvantage throws two dice, keeps the lower one and adds the modifier', () => {
    const result = rollExpression(expression('1d20 + 3'), {
      rowKey: 'combat.initiative',
      values: {},
      words: wordsFor([5, 17]),
      keep: 'lowest',
    });

    expect(result.ok && result.value).toEqual({
      total: 8,
      parts: [
        { sign: 1, label: '1d20', value: 5, rolls: [5, 17], discarded: [1] },
        { sign: 1, label: '3', value: 3 },
      ],
    });
  });

  it('Edge: both dice equal keep the first and the total is still correct', () => {
    const result = rollExpression(expression('1d20 + 3'), {
      rowKey: 'combat.initiative',
      values: {},
      words: wordsFor([8, 8]),
      keep: 'highest',
    });

    expect(result.ok && result.value.total).toBe(11);
    expect(result.ok && result.value.parts[0]).toEqual({
      sign: 1,
      label: '1d20',
      value: 8,
      rolls: [8, 8],
      discarded: [1],
    });
  });

  it('Edge: a negative modifier subtracts after the kept die is chosen', () => {
    const result = rollExpression(expression('1d20 - 3'), {
      rowKey: 'combat.initiative',
      values: {},
      words: wordsFor([5, 17]),
      keep: 'lowest',
    });

    expect(result.ok && result.value.total).toBe(2);
    expect(result.ok && result.value.parts[0].value).toBe(5);
  });

  it('Edge: a keep mode leaves dice that are not d20 alone', () => {
    const words = wordsFor([3, 5, 5, 17]);
    const result = rollExpression(expression('2d6 + 1d20'), {
      rowKey: 'x',
      values: {},
      words,
      keep: 'highest',
    });

    expect(result.ok && result.value.parts[0]).toEqual({
      sign: 1,
      label: '2d6',
      value: 8,
      rolls: [3, 5],
    });
    expect(result.ok && result.value.parts[1]).toEqual({
      sign: 1,
      label: '1d20',
      value: 17,
      rolls: [5, 17],
      discarded: [0],
    });
    expect(words.calls()).toBe(4);
  });

  it('Edge: 2d20 with advantage throws three dice and keeps the two highest', () => {
    const result = rollExpression(expression('2d20'), {
      rowKey: 'x',
      values: {},
      words: wordsFor([2, 20, 10]),
      keep: 'highest',
    });

    expect(result.ok && result.value).toEqual({
      total: 30,
      parts: [{ sign: 1, label: '2d20', value: 30, rolls: [2, 20, 10], discarded: [0] }],
    });
  });

  it('Edge: no keep mode throws exactly the number of dice the term names', () => {
    const words = wordsFor([5, 17, 9]);
    rollExpression(expression('1d20'), { rowKey: 'x', values: {}, words });
    expect(words.calls()).toBe(1);
  });

  it('Edge: a signed term keeps its sign and keeps the highest die', () => {
    const result = rollExpression(expression('-1d20 + 10'), {
      rowKey: 'x',
      values: {},
      words: wordsFor([5, 17]),
      keep: 'highest',
    });

    expect(result.ok && result.value.total).toBe(-7);
    expect(result.ok && result.value.parts[0]).toEqual({
      sign: -1,
      label: '1d20',
      value: 17,
      rolls: [5, 17],
      discarded: [0],
    });
  });

  it('Edge: the default source rolls two dice under a keep mode', () => {
    let drawn = 0;
    const result = rollExpression(expression('1d20'), {
      rowKey: 'x',
      values: {},
      words: () => {
        drawn += 1;
        return drawn;
      },
      keep: 'lowest',
    });

    expect(result.ok).toBe(true);
    expect(drawn).toBe(2);
  });
});

describe('performRoll with a keep mode', () => {
  const schema = schemaFromKinds({});
  const choices: DiceChoices = {};

  const roll = (keep: 'highest' | 'lowest') => {
    const resolution = resolveDice({
      rowKey: 'combat.initiative',
      sectionKey: 'combat',
      schema,
      template: { rowDice: { 'combat.initiative': '1d20 + 2' } },
      choices,
    });
    return performRoll(resolution, {
      rowKey: 'combat.initiative',
      values: { combat: { initiative: 2 } },
      words: wordsFor([5, 17]),
      keep,
    });
  };

  it('Happy path: advantage keeps the higher die through the whole resolution', () => {
    const result = roll('highest');
    expect(result.ok && result.value.total).toBe(19);
  });

  it('Happy path: disadvantage keeps the lower die through the whole resolution', () => {
    const result = roll('lowest');
    expect(result.ok && result.value.total).toBe(7);
  });
});
