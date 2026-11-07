import { describe, expect, it } from 'vitest';
import {
  resolveDice,
  schemaFromKinds,
  type DiceChoices,
  type DiceDefaults,
  type Resolution,
} from '../dice';
import { diceVectors } from './dice_vectors';

const schema = schemaFromKinds(diceVectors.fields);

const ABILITIES = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'];

/** The slice of `fantasy_v1` the prototype walks through. */
const template: DiceDefaults = Object.freeze({
  sectionDice: Object.freeze({
    skills: '1d20 + @.total',
    saves: '1d20 + @',
    attacks: '1d20 + @.bonus',
  }),
  rowDice: Object.freeze({
    'combat.initiative': '1d20 + @',
    ...Object.fromEntries(ABILITIES.map((a) => [`stats.${a}`, `1d20 + modifiers.${a}Mod`])),
  }),
});

function resolve(
  rowKey: string,
  sectionKey: string,
  choices: DiceChoices = {},
  defaults: DiceDefaults = template,
): Resolution {
  return resolveDice({ rowKey, sectionKey, schema, template: defaults, choices });
}

describe('resolveDice precedence', () => {
  it('Happy path: step 5, the Template Section expression serves a Row with nothing else', () => {
    const resolved = resolve('skills.acrobatics', 'skills');
    expect(resolved).toMatchObject({
      source: 'template-section',
      expression: '1d20 + @.total',
      rollable: true,
    });
  });

  it('Happy path: step 3, a Template Row expression serves its Row', () => {
    expect(resolve('combat.initiative', 'combat')).toMatchObject({
      source: 'template-row',
      expression: '1d20 + @',
      rollable: true,
    });
  });

  it('Happy path: step 4, the player Section expression replaces the Template Section one', () => {
    const resolved = resolve('skills.acrobatics', 'skills', {
      section: { skills: '1d20 + @.total + 1' },
    });
    expect(resolved).toMatchObject({
      source: 'sheet-section',
      expression: '1d20 + @.total + 1',
      rollable: true,
    });
  });

  it('Happy path: step 4 serves a Section the Template gave nothing', () => {
    const resolved = resolve('combat.hitPoints', 'combat', { section: { combat: '1d20 + @' } });
    expect(resolved).toMatchObject({ source: 'sheet-section', rollable: true });
  });

  it('Edge: the Template Row expression beats the player Section expression', () => {
    const resolved = resolve('stats.strength', 'stats', { section: { stats: '1d20 + @' } });
    expect(resolved).toMatchObject({
      source: 'template-row',
      expression: '1d20 + modifiers.strengthMod',
    });
  });

  it('Edge: step 1, the player Row expression beats everything', () => {
    const resolved = resolve('stats.strength', 'stats', {
      row: { 'stats.strength': '2d6' },
      section: { stats: null },
    });
    expect(resolved).toMatchObject({ source: 'sheet-row', expression: '2d6', rollable: true });
  });

  it('Edge: step 2, "no dice" on the Section silences a Template Row default', () => {
    const resolved = resolve('stats.strength', 'stats', { section: { stats: null } });
    expect(resolved).toMatchObject({
      source: 'sheet-section-off',
      expression: null,
      rollable: false,
      reason: { class: 'turned_off' },
    });
  });

  it('Edge: "no dice" on the Section silences the Template Section default too', () => {
    const resolved = resolve('skills.acrobatics', 'skills', { section: { skills: null } });
    expect(resolved).toMatchObject({ source: 'sheet-section-off', rollable: false });
  });

  it('Edge: a Row with its own player expression still rolls when its Section is off', () => {
    const resolved = resolve('skills.perception', 'skills', {
      row: { 'skills.perception': '1d20 + @.total' },
      section: { skills: null },
    });
    expect(resolved).toMatchObject({ source: 'sheet-row', rollable: true });
  });

  it('Edge: "no dice" on a Row beats every default and Section choice', () => {
    const resolved = resolve('combat.initiative', 'combat', {
      row: { 'combat.initiative': null },
      section: { combat: '1d20 + @' },
    });
    expect(resolved).toMatchObject({
      source: 'sheet-row-off',
      expression: null,
      rollable: false,
      reason: { class: 'turned_off' },
    });
  });

  it('Edge: with no dice anywhere the Row is not rollable', () => {
    const resolved = resolve('combat.hitPoints', 'combat');
    expect(resolved).toMatchObject({
      source: 'none',
      expression: null,
      rollable: false,
      reason: { class: 'no_dice' },
    });
  });

  it('Edge: with no Template defaults at all every Row falls to "none"', () => {
    expect(resolve('combat.initiative', 'combat', {}, {})).toMatchObject({ source: 'none' });
  });
});

describe('resolveDice absent versus null', () => {
  it('Edge: an absent Row key falls through while a null one switches rolling off', () => {
    const absent = resolve('combat.initiative', 'combat', { row: {} });
    const off = resolve('combat.initiative', 'combat', { row: { 'combat.initiative': null } });
    expect(absent.source).toBe('template-row');
    expect(off.source).toBe('sheet-row-off');
  });

  it('Edge: an absent Section key falls through while a null one switches rolling off', () => {
    const absent = resolve('stats.strength', 'stats', { section: {} });
    const off = resolve('stats.strength', 'stats', { section: { stats: null } });
    expect(absent.source).toBe('template-row');
    expect(off.source).toBe('sheet-section-off');
  });

  it('Edge: a key holding undefined counts as absent, not as "no dice"', () => {
    const resolved = resolve('combat.initiative', 'combat', {
      row: { 'combat.initiative': undefined as unknown as null },
    });
    expect(resolved.source).toBe('template-row');
  });

  it('Edge: inherited object keys are never read as choices', () => {
    expect(resolve('toString', 'constructor').source).toBe('none');
    expect(resolve('__proto__', '__proto__').source).toBe('none');
  });
});

describe('resolveDice fitting', () => {
  it('Error: a player expression whose @ is no numeric Field is not rollable, and nothing replaces it', () => {
    const resolved = resolve('combat.initiative', 'combat', {
      row: { 'combat.initiative': '1d20 + @.total' },
    });
    expect(resolved).toMatchObject({
      source: 'sheet-row',
      expression: '1d20 + @.total',
      rollable: false,
      reason: { class: 'unknown_field' },
    });
  });

  it('Error: a Section expression is judged Row by Row', () => {
    const choices: DiceChoices = { section: { modifiers: '1d20 + @.total' } };
    expect(resolve('modifiers.strengthMod', 'modifiers', choices)).toMatchObject({
      source: 'sheet-section',
      rollable: false,
      reason: { class: 'unknown_field' },
    });
    expect(resolve('skills.acrobatics', 'modifiers', choices).rollable).toBe(true);
  });

  it('Error: a Field the Template no longer has makes the expression not rollable, never a throw', () => {
    const resolved = resolve('combat.initiative', 'combat', {
      row: { 'combat.initiative': '1d20 + retired.field' },
    });
    expect(resolved).toMatchObject({ rollable: false, reason: { class: 'unknown_field' } });
  });

  it('Error: a text Field in the expression is not numeric', () => {
    const resolved = resolve('combat.initiative', 'combat', {
      row: { 'combat.initiative': '1d20 + identity.name' },
    });
    expect(resolved).toMatchObject({ rollable: false, reason: { class: 'non_numeric_field' } });
  });

  it('Error: a stored expression the grammar refuses is not rollable and carries the parse class', () => {
    const resolved = resolve('combat.initiative', 'combat', {
      row: { 'combat.initiative': '1d20 * 2' },
    });
    expect(resolved).toMatchObject({
      source: 'sheet-row',
      rollable: false,
      reason: { class: 'unreadable_term' },
    });
  });

  it('Edge: a rollable resolution carries the parsed expression', () => {
    const resolved = resolve('skills.acrobatics', 'skills');
    expect(resolved.rollable && resolved.parsed.terms).toHaveLength(2);
  });
});

describe('resolveDice stale choices', () => {
  const stale: DiceChoices = {
    row: { 'stats.luck': '1d20', 'gone.row': null },
    section: { vanished: '1d20', retired: null },
  };

  it('Edge: choices for Rows and Sections the Template no longer has are ignored', () => {
    expect(resolve('skills.acrobatics', 'skills', stale)).toMatchObject({
      source: 'template-section',
      rollable: true,
    });
    expect(resolve('combat.hitPoints', 'combat', stale).source).toBe('none');
  });
});

describe('resolveDice purity', () => {
  it('Edge: frozen inputs are read, never written', () => {
    const choices: DiceChoices = Object.freeze({
      row: Object.freeze({ 'skills.athletics': '1d20 + @.total + 2' }),
      section: Object.freeze({ stats: null }),
    });
    expect(() => resolve('skills.athletics', 'skills', choices)).not.toThrow();
    expect(() => resolve('stats.strength', 'stats', choices)).not.toThrow();
  });
});
