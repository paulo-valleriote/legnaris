import type { MessageKey } from '../../../l10n/strings';
import type { AbilityKey } from './abilities_data';
import { parseClampedInt } from './parse_number';

export const SKILL_KEYS = [
  'athletics',
  'acrobatics',
  'stealth',
  'perception',
  'insight',
  'survival',
  'persuasion',
  'deception',
  'intimidation',
  'knowledge',
  'analysis',
  'craft',
] as const;

export type SkillKey = (typeof SKILL_KEYS)[number];

const SKILL_ABILITIES: Readonly<Record<SkillKey, AbilityKey>> = {
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

const ABILITY_ABBREVIATIONS: Readonly<Record<AbilityKey, MessageKey>> = {
  strength: 'sheet.play.ability.strength',
  dexterity: 'sheet.play.ability.dexterity',
  constitution: 'sheet.play.ability.constitution',
  intelligence: 'sheet.play.ability.intelligence',
  wisdom: 'sheet.play.ability.wisdom',
  charisma: 'sheet.play.ability.charisma',
};

export const SKILL_BONUS_MIN = -10;
export const SKILL_BONUS_MAX = 20;

export function skillAbility(skill: SkillKey): AbilityKey {
  return SKILL_ABILITIES[skill];
}

export function abilityAbbreviationKey(ability: AbilityKey): MessageKey {
  return ABILITY_ABBREVIATIONS[ability];
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

export function saveTotalPath(ability: AbilityKey): string {
  return `saves.${ability}`;
}

export function saveProficiencyPath(ability: AbilityKey): string {
  return `config.save${capitalize(ability)}Prof`;
}

export function skillRowKey(skill: SkillKey): string {
  return `skills.${skill}`;
}

export function skillBonusPath(skill: SkillKey): string {
  return `skills.${skill}.bonus`;
}

export function skillTotalPath(skill: SkillKey): string {
  return `skills.${skill}.total`;
}

export function skillProficiencyPath(skill: SkillKey): string {
  return `config.skill${capitalize(skill)}Prof`;
}

export function isProficient(value: unknown): boolean {
  return typeof value === 'number' && Number.isFinite(value) && value !== 0;
}

export function nextProficiency(value: unknown): 0 | 1 {
  return isProficient(value) ? 0 : 1;
}

export function parseSkillBonus(text: string): number | undefined {
  return parseClampedInt(text, SKILL_BONUS_MIN, SKILL_BONUS_MAX);
}
