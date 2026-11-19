import type { MessageKey } from '../../../l10n/strings';

export const ARMOR_CLASS_PATH = 'derived.armorClass';
export const BASE_ARMOR_PATH = 'combat.baseArmorClass';
export const INITIATIVE_PATH = 'combat.initiative';
export const SPEED_PATH = 'combat.speed';

export const BASE_ARMOR_MIN = 1;
export const BASE_ARMOR_MAX = 30;
export const SPEED_MIN = 0;
export const SPEED_MAX = 120;

export const ARMOR_ABILITY_KEYS = ['dex', 'con'] as const;

export type ArmorAbilityKey = (typeof ARMOR_ABILITY_KEYS)[number];

export interface ArmorAbility {
  readonly key: ArmorAbilityKey;
  readonly configPath: string;
  readonly modifierPath: string;
  readonly shortLabel: MessageKey;
}

export const ARMOR_ABILITIES: Readonly<Record<ArmorAbilityKey, ArmorAbility>> = {
  dex: {
    key: 'dex',
    configPath: 'config.acUsesDex',
    modifierPath: 'modifiers.dexterityMod',
    shortLabel: 'sheet.play.acDex',
  },
  con: {
    key: 'con',
    configPath: 'config.acUsesCon',
    modifierPath: 'modifiers.constitutionMod',
    shortLabel: 'sheet.play.acCon',
  },
};

export type ArmorAbilityValues = Readonly<Partial<Record<ArmorAbilityKey, unknown>>>;

function isOn(value: unknown): boolean {
  return typeof value === 'number' && value !== 0;
}

export function armorAbilityConfigPaths(): readonly string[] {
  return ARMOR_ABILITY_KEYS.map((key) => ARMOR_ABILITIES[key].configPath);
}

export function armorAbilityFromValues(values: ArmorAbilityValues): ArmorAbilityKey | undefined {
  const on = ARMOR_ABILITY_KEYS.filter((key) => isOn(values[key]));
  return on.length === 1 ? on[0] : undefined;
}

function clampInteger(text: string, min: number, max: number): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  return Math.min(max, Math.max(min, Math.round(parsed)));
}

export function parseBaseArmor(text: string): number | undefined {
  return clampInteger(text, BASE_ARMOR_MIN, BASE_ARMOR_MAX);
}

export function parseSpeed(text: string): number | undefined {
  return clampInteger(text, SPEED_MIN, SPEED_MAX);
}
