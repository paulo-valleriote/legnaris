import { parseClampedInt } from './parse_number';

export const ABILITY_KEYS = [
  'strength',
  'dexterity',
  'constitution',
  'intelligence',
  'wisdom',
  'charisma',
] as const;

export type AbilityKey = (typeof ABILITY_KEYS)[number];

export const ABILITY_SCORE_MIN = 3;
export const ABILITY_SCORE_MAX = 20;

export function abilityScorePath(ability: AbilityKey): string {
  return `stats.${ability}`;
}

export function abilityModifierPath(ability: AbilityKey): string {
  return `modifiers.${ability}Mod`;
}

export function formatModifier(modifier: number): string {
  return modifier >= 0 ? `+${modifier}` : String(modifier);
}

export function parseAbilityScore(text: string): number | undefined {
  return parseClampedInt(text, ABILITY_SCORE_MIN, ABILITY_SCORE_MAX);
}
