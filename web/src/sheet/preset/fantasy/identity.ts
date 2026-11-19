import { parseClampedInt } from './parse_number';

export const PROFICIENCY_BONUS_MIN = 2;

export const PROFICIENCY_BONUS_MAX = 6;

export function parseProficiencyBonus(text: string): number | undefined {
  return parseClampedInt(text, PROFICIENCY_BONUS_MIN, PROFICIENCY_BONUS_MAX);
}
