import { describe, expect, it } from 'vitest';
import { ABILITY_KEYS } from '../sheet/preset/fantasy/abilities_data';
import {
  abilityAbbreviationKey,
  isProficient,
  nextProficiency,
  parseSkillBonus,
  SKILL_BONUS_MAX,
  SKILL_BONUS_MIN,
  SKILL_KEYS,
  saveProficiencyPath,
  saveTotalPath,
  skillAbility,
  skillBonusPath,
  skillProficiencyPath,
  skillRowKey,
  skillTotalPath,
  type SkillKey,
} from '../sheet/preset/fantasy/saves_skills_data';
import { message } from '../l10n/strings';

describe('saving throw paths', () => {
  it('Edge: every ability maps to a distinct save total and a capitalized config leaf', () => {
    const totals = new Set<string>();
    const leaves = new Set<string>();
    for (const ability of ABILITY_KEYS) {
      totals.add(saveTotalPath(ability));
      leaves.add(saveProficiencyPath(ability));
    }

    expect(totals.size).toBe(ABILITY_KEYS.length);
    expect(leaves.size).toBe(ABILITY_KEYS.length);
    expect(saveTotalPath('strength')).toBe('saves.strength');
    expect(saveProficiencyPath('strength')).toBe('config.saveStrengthProf');
    expect(saveProficiencyPath('constitution')).toBe('config.saveConstitutionProf');
    expect(saveProficiencyPath('charisma')).toBe('config.saveCharismaProf');
  });
});

describe('skill paths', () => {
  it('Edge: all twelve skills expose distinct bonus, total and proficiency paths', () => {
    expect(SKILL_KEYS).toHaveLength(12);
    expect(new Set(SKILL_KEYS).size).toBe(SKILL_KEYS.length);

    const bonuses = new Set(SKILL_KEYS.map(skillBonusPath));
    const totals = new Set(SKILL_KEYS.map(skillTotalPath));
    const leaves = new Set(SKILL_KEYS.map(skillProficiencyPath));

    expect(bonuses.size).toBe(SKILL_KEYS.length);
    expect(totals.size).toBe(SKILL_KEYS.length);
    expect(leaves.size).toBe(SKILL_KEYS.length);
    expect(skillBonusPath('athletics')).toBe('skills.athletics.bonus');
    expect(skillRowKey('athletics')).toBe('skills.athletics');
    expect(skillTotalPath('athletics')).toBe('skills.athletics.total');
    expect(skillProficiencyPath('athletics')).toBe('config.skillAthleticsProf');
    expect(skillProficiencyPath('knowledge')).toBe('config.skillKnowledgeProf');
  });

  it('Happy path: every skill points at one of the six abilities', () => {
    const expected: Readonly<Record<SkillKey, string>> = {
      athletics: 'strength',
      acrobatics: 'dexterity',
      stealth: 'dexterity',
      perception: 'wisdom',
      insight: 'wisdom',
      survival: 'wisdom',
      persuasion: 'charisma',
      deception: 'charisma',
      intimidation: 'charisma',
      knowledge: 'intelligence',
      analysis: 'intelligence',
      craft: 'intelligence',
    };

    for (const skill of SKILL_KEYS) {
      expect(ABILITY_KEYS).toContain(skillAbility(skill));
      expect(skillAbility(skill)).toBe(expected[skill]);
    }
  });
});

describe('ability abbreviations', () => {
  it('Edge: every ability resolves a distinct three-letter abbreviation', () => {
    const abbreviations = ABILITY_KEYS.map((ability) => message(abilityAbbreviationKey(ability)));

    expect(abbreviations).toEqual(['STR', 'DEX', 'CON', 'INT', 'WIS', 'CHA']);
    expect(new Set(abbreviations).size).toBe(ABILITY_KEYS.length);
  });
});

describe('isProficient', () => {
  it('Edge: a missing, non-numeric, or corrupt value reads as not proficient', () => {
    const values: unknown[] = [undefined, null, '', '1', false, {}, [], Number.NaN];
    for (const value of values) {
      expect(isProficient(value)).toBe(false);
    }
  });

  it('Edge: zero reads as not proficient and any other finite number reads as proficient', () => {
    expect(isProficient(0)).toBe(false);
    expect(isProficient(-0)).toBe(false);
    expect(isProficient(1)).toBe(true);
    expect(isProficient(2)).toBe(true);
    expect(isProficient(-1)).toBe(true);
    expect(isProficient(Number.POSITIVE_INFINITY)).toBe(false);
  });
});

describe('nextProficiency', () => {
  it('Edge: an unpressed dot turns on and a pressed dot turns off', () => {
    expect(nextProficiency(0)).toBe(1);
    expect(nextProficiency(undefined)).toBe(1);
    expect(nextProficiency(1)).toBe(0);
    expect(nextProficiency(7)).toBe(0);
  });
});

describe('parseSkillBonus', () => {
  it.each([
    ['', undefined],
    ['   ', undefined],
    ['abc', undefined],
    ['-', undefined],
    ['.', undefined],
    ['1e999', undefined],
  ])('Edge: %j resolves to %j', (text, expected) => {
    expect(parseSkillBonus(text)).toBe(expected);
  });

  it.each([
    ['0', 0],
    ['3', 3],
    ['+3', 3],
    ['-3', -3],
  ])('Happy path: %j resolves to %j', (text, expected) => {
    expect(parseSkillBonus(text)).toBe(expected);
  });

  it.each([
    ['1.4', 1],
    ['1.6', 2],
  ])('Edge: %j rounds to %j', (text, expected) => {
    expect(parseSkillBonus(text)).toBe(expected);
  });

  it('Edge: both bounds are inclusive and anything past them clamps', () => {
    expect(parseSkillBonus(String(SKILL_BONUS_MIN))).toBe(SKILL_BONUS_MIN);
    expect(parseSkillBonus(String(SKILL_BONUS_MAX))).toBe(SKILL_BONUS_MAX);
    expect(parseSkillBonus(String(SKILL_BONUS_MIN - 1))).toBe(SKILL_BONUS_MIN);
    expect(parseSkillBonus(String(SKILL_BONUS_MAX + 1))).toBe(SKILL_BONUS_MAX);
  });
});
