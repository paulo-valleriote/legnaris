export const PROFICIENCY_BONUS_MIN = 2;

export const PROFICIENCY_BONUS_MAX = 6;

export function parseProficiencyBonus(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  return Math.min(PROFICIENCY_BONUS_MAX, Math.max(PROFICIENCY_BONUS_MIN, Math.round(parsed)));
}
