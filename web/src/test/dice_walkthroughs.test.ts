import { describe, expect, it } from 'vitest';
import {
  evaluateExpression,
  resolveDice,
  schemaFromKinds,
  validateRow,
  validateSection,
  type DiceChoices,
  type DiceDefaults,
  type DiceResult,
  type RollResult,
} from '../dice';
import { diceVectors } from './dice_vectors';

/** The six walkthroughs of the LGN-161 prototype, replayed through resolution and Roll with every die at its highest face. */

const schema = schemaFromKinds(diceVectors.fields);

const template: DiceDefaults = {
  sectionDice: {
    skills: '1d20 + @.total',
    saves: '1d20 + @',
    attacks: '1d20 + @.bonus',
  },
  rowDice: {
    'combat.initiative': '1d20 + @',
    'stats.strength': '1d20 + modifiers.strengthMod',
    'stats.dexterity': '1d20 + modifiers.dexterityMod',
  },
};

interface Sheet {
  dexterity: number;
  acrobaticsBonus: number;
}

/** The slice of the sheet the walkthroughs read, derived the way the Template formulas do. */
function sheetOf({ dexterity, acrobaticsBonus }: Sheet): Record<string, unknown> {
  const modifier = (score: number) => Math.floor((score - 10) / 2);
  return {
    stats: { strength: 10, dexterity, wisdom: 14 },
    modifiers: { strengthMod: modifier(10), dexterityMod: modifier(dexterity) },
    skills: {
      acrobatics: { bonus: acrobaticsBonus, total: modifier(dexterity) + acrobaticsBonus },
      athletics: { bonus: 0, total: modifier(10) },
      perception: { bonus: 0, total: modifier(14) },
    },
    combat: { initiative: 2, hitPoints: 10 },
  };
}

function roll(
  rowKey: string,
  sectionKey: string,
  choices: DiceChoices,
  sheet: Record<string, unknown> = sheetOf({ dexterity: 16, acrobaticsBonus: 0 }),
  roller: (faces: number) => number = (faces) => faces,
): DiceResult<RollResult> {
  const resolution = resolveDice({ rowKey, sectionKey, schema, template, choices });
  if (!resolution.rollable) {
    return { ok: false, error: resolution.reason };
  }
  return evaluateExpression(resolution.parsed, { rowKey, values: sheet, roller });
}

const totalOf = (result: DiceResult<RollResult>) =>
  result.ok ? result.value.total : result.error.class;

describe('walkthrough 1: happy path', () => {
  it('Happy path: Section default, Row default, a stat reading another Section, and a Row with no dice', () => {
    expect(totalOf(roll('skills.acrobatics', 'skills', {}))).toBe(23);
    expect(totalOf(roll('combat.initiative', 'combat', {}))).toBe(22);
    expect(totalOf(roll('stats.dexterity', 'stats', {}))).toBe(23);
    expect(totalOf(roll('combat.hitPoints', 'combat', {}))).toBe('no_dice');
  });
});

describe('walkthrough 2: the player changes a Row', () => {
  it('Happy path: a flat bonus on one Row leaves the others alone, and going back restores the Template', () => {
    const bonus: DiceChoices = { row: { 'skills.athletics': '1d20 + @.total + 2' } };
    expect(totalOf(roll('skills.athletics', 'skills', bonus))).toBe(22);
    expect(totalOf(roll('skills.acrobatics', 'skills', bonus))).toBe(23);
    expect(totalOf(roll('skills.athletics', 'skills', {}))).toBe(20);
  });
});

describe('walkthrough 3: a Section choice against a Template Row', () => {
  it('Happy path: the Template Row wins, the Section choice serves the Rows without one', () => {
    expect(totalOf(roll('stats.strength', 'stats', {}))).toBe(20);
    expect(totalOf(roll('stats.strength', 'stats', { section: { stats: '1d20 + @' } }))).toBe(20);
    expect(
      totalOf(roll('skills.acrobatics', 'skills', { section: { skills: '1d20 + @.total + 1' } })),
    ).toBe(24);
    expect(totalOf(roll('combat.hitPoints', 'combat', { section: { combat: '1d20 + @' } }))).toBe(
      30,
    );
  });

  it('Happy path: the attack column serves a table Row', () => {
    const sheet = {
      ...sheetOf({ dexterity: 16, acrobaticsBonus: 0 }),
      attacks: { attack1: { bonus: 5 } },
    };
    expect(totalOf(roll('attacks.attack1', 'attacks', {}, sheet, () => 10))).toBe(15);
  });
});

describe('walkthrough 4: turning dice off', () => {
  it('Edge: "no dice" on a Row or a Section refuses the Roll, and a Row choice beats a Section that is off', () => {
    expect(
      totalOf(roll('combat.initiative', 'combat', { row: { 'combat.initiative': null } })),
    ).toBe('turned_off');
    const skillsOff: DiceChoices = { section: { skills: null } };
    expect(totalOf(roll('skills.acrobatics', 'skills', skillsOff))).toBe('turned_off');
    const ownPerception: DiceChoices = {
      ...skillsOff,
      row: { 'skills.perception': '1d20 + @.total' },
    };
    expect(totalOf(roll('skills.perception', 'skills', ownPerception))).toBe(22);
    const statsOff: DiceChoices = { section: { stats: null } };
    expect(totalOf(roll('stats.strength', 'stats', statsOff))).toBe('turned_off');
    expect(totalOf(roll('stats.strength', 'stats', {}))).toBe(20);
  });
});

describe('walkthrough 5: bad input', () => {
  it.each([
    ['1d20 +', 'missing_term'],
    ['21d6', 'dice_count'],
    ['1d1001', 'dice_faces'],
    ['1d20 + 10000', 'number_range'],
    ['0d6', 'dice_count'],
    ['1d20 * 2', 'unreadable_term'],
    ['1d20 + identity.name', 'non_numeric_field'],
    ['1d20 + nope.nothing', 'unknown_field'],
    ['1d20 + @.total', 'unknown_field'],
    ['1+1+1+1+1+1+1+1+1', 'too_many_terms'],
    ['1d20  +  9999  +  9999  +  9999  +  9999  +  9999  +  9999  +  9999', 'too_long'],
  ])('Error: "%s" on Initiative is refused as %s', (expr, reason) => {
    const result = validateRow(schema, 'combat.initiative', expr);
    expect(!result.ok && result.error.class).toBe(reason);
  });

  it('Error: "@.total" as the Modifiers Section default fits no Row', () => {
    const rows = ['modifiers.strengthMod', 'modifiers.dexterityMod', 'modifiers.constitutionMod'];
    const result = validateSection(schema, rows, '1d20 + @.total');
    expect(!result.ok && result.error.class).toBe('unknown_field');
  });

  it('Happy path: "1d20 - 1" and "d20" are accepted, and "d20" rolls as 1d20', () => {
    expect(validateRow(schema, 'combat.initiative', '1d20 - 1').ok).toBe(true);
    expect(
      totalOf(roll('combat.initiative', 'combat', { row: { 'combat.initiative': 'd20' } })),
    ).toBe(20);
  });
});

describe('walkthrough 6: what a Roll reads', () => {
  it('Happy path: the modifier follows the stat, fractions are floored, totals can go negative', () => {
    expect(totalOf(roll('skills.acrobatics', 'skills', {}))).toBe(23);
    const strongerDexterity = sheetOf({ dexterity: 20, acrobaticsBonus: 0 });
    expect(totalOf(roll('skills.acrobatics', 'skills', {}, strongerDexterity))).toBe(25);
    const fractionalBonus = sheetOf({ dexterity: 20, acrobaticsBonus: 2.7 });
    expect(totalOf(roll('skills.acrobatics', 'skills', {}, fractionalBonus))).toBe(27);
    const negative = roll(
      'combat.initiative',
      'combat',
      { row: { 'combat.initiative': '1d4 - 5' } },
      undefined,
      () => 1,
    );
    expect(totalOf(negative)).toBe(-4);
  });
});
