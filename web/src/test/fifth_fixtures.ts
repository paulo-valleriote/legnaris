import characterGolden from '../../../testdata/templates/fifth_character_v1.json';
import statBlockGolden from '../../../testdata/templates/fifth_stat_block_v1.json';
import type { Sheet } from '../api/models/sheet';
import { parseSheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';

export const FIFTH_CHARACTER_TEMPLATE_ID = 'fifth_character_v1';
export const FIFTH_STAT_BLOCK_TEMPLATE_ID = 'fifth_stat_block_v1';

export const FIFTH_CHARACTER_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd71';
export const FIFTH_STAT_BLOCK_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd72';

const RECORD = 'r0123456789abcdef0123456789abcdef';
const RECORD_B = 'r0123456789abcdef0123456789abcdee';

const ABILITY_KEYS = [
  'strength',
  'dexterity',
  'constitution',
  'intelligence',
  'wisdom',
  'charisma',
] as const;

const SKILL_KEYS = [
  'acrobatics',
  'animalHandling',
  'arcana',
  'athletics',
  'deception',
  'history',
  'insight',
  'intimidation',
  'investigation',
  'medicine',
  'nature',
  'perception',
  'performance',
  'persuasion',
  'religion',
  'sleightOfHand',
  'stealth',
  'survival',
] as const;

const CONDITION_KEYS = [
  'blinded',
  'charmed',
  'deafened',
  'frightened',
  'grappled',
  'incapacitated',
  'invisible',
  'paralyzed',
  'petrified',
  'poisoned',
  'prone',
  'restrained',
  'stunned',
  'unconscious',
] as const;

const SLOT_LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

export function fifthCharacterTemplatePayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return { ...characterGolden.data, ...overrides };
}

export function fifthStatBlockTemplatePayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return { ...statBlockGolden.data, ...overrides };
}

export function fifthCharacterSheetPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: FIFTH_CHARACTER_SHEET_ID,
    template_id: FIFTH_CHARACTER_TEMPLATE_ID,
    kind: 'character',
    name: 'Brenna',
    data: fifthCharacterSheetData(),
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
    ...overrides,
  };
}

export function fifthStatBlockSheetPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: FIFTH_STAT_BLOCK_SHEET_ID,
    template_id: FIFTH_STAT_BLOCK_TEMPLATE_ID,
    kind: 'npc',
    name: 'Gray Ooze',
    data: fifthStatBlockSheetData(),
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
    ...overrides,
  };
}

export function fifthCharacterTemplate(): TemplateDetail {
  return parseTemplateDetail(characterGolden.data);
}

export function fifthStatBlockTemplate(): TemplateDetail {
  return parseTemplateDetail(statBlockGolden.data);
}

function zeroAbilities(): Record<string, unknown> {
  return Object.fromEntries(ABILITY_KEYS.map((key) => [key, { score: 10, mod: 0 }]));
}

function saves(): Record<string, unknown> {
  return Object.fromEntries(ABILITY_KEYS.map((key) => [key, { prof: 0, misc: 0, total: 0 }]));
}

function skills(): Record<string, unknown> {
  return Object.fromEntries(SKILL_KEYS.map((key) => [key, { rank: 0, misc: 0, total: 0 }]));
}

function conditions(): Record<string, unknown> {
  return Object.fromEntries(CONDITION_KEYS.map((key) => [key, 0]));
}

function spellSlots(): Record<string, unknown> {
  const slots: Record<string, unknown> = Object.fromEntries(
    SLOT_LEVELS.map((level) => [`slot${level}`, { total: 1, spent: 0, remaining: 1 }]),
  );
  slots.pact = { count: 1, level: 1, used: 0, remaining: 1 };
  return slots;
}

export function fifthCharacterSheetData(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    identity: {
      name: 'Brenna',
      class: 'Ranger',
      subclass: 'Hunter',
      species: 'Human',
      background: 'Outlander',
      level: 5,
      proficiencyBonus: 3,
      jackOfAllTrades: 0,
    },
    abilities: zeroAbilities(),
    saves: saves(),
    skills: skills(),
    exhaustion: { level: 0, penalty: 0 },
    passive: {
      choiceNone: 1,
      choiceAdvantage: 0,
      choiceDisadvantage: 0,
      perception: 10,
    },
    armor: {
      choiceNone: 1,
      choiceLight: 0,
      choiceMedium: 0,
      choiceHeavy: 0,
      choiceBarbarian: 0,
      choiceMonk: 0,
      base: 10,
      shield: 0,
      misc: 0,
      armorClass: 10,
    },
    hitDice: {
      primary: { size: 10, count: 5, spent: 0, remaining: 5 },
      d6: { extra: 1, spent: 0, remaining: 1 },
      d8: { extra: 1, spent: 0, remaining: 1 },
      d10: { extra: 1, spent: 0, remaining: 1 },
      d12: { extra: 1, spent: 0, remaining: 1 },
    },
    spellcasting: {
      abilityStrength: 0,
      abilityDexterity: 0,
      abilityConstitution: 0,
      abilityIntelligence: 0,
      abilityWisdom: 1,
      abilityCharisma: 0,
      abilityMod: 0,
      attack: 0,
      saveDC: 0,
      preparedLimit: 0,
      ...spellSlots(),
    },
    combat: {
      initiative: { alert: 0, misc: 0, total: 0 },
      speed: { base: 30, total: 30 },
      armorStrengthRequirement: 0,
      hp: { base: 0, perLevel: 0, misc: 0, max: 0, current: 0, temp: 0 },
      carryingCapacity: 0,
    },
    status: {
      xp: 0,
      heroicInspiration: 0,
      conditions: conditions(),
      deathSaves: { successes: 0, failures: 0 },
    },
    coins: { cp: 0, sp: 0, ep: 0, gp: 0, pp: 0 },
    attacks: {
      list: {
        [RECORD]: {
          abilityStrength: 1,
          abilityDexterity: 0,
          abilityConstitution: 0,
          abilityIntelligence: 0,
          abilityWisdom: 0,
          abilityCharisma: 0,
          proficient: 1,
          misc: 0,
          toHit: 0,
          damageBonus: 0,
          name: 'Longbow',
          damage: '1d8 + 1',
          notes: '',
          position: 0,
        },
      },
    },
    inventory: {
      items: {
        [RECORD]: {
          name: 'Rope',
          quantity: 1,
          weight: 5,
          lineWeight: 5,
          attuned: 0,
          notes: '',
          position: 0,
        },
        [RECORD_B]: {
          name: 'Torch',
          quantity: 3,
          weight: 1,
          lineWeight: 3,
          attuned: 0,
          notes: '',
          position: 1,
        },
      },
      totalWeight: 8,
      attuned: 0,
    },
    features: {
      list: { [RECORD]: { name: 'Favored Enemy', source: 'Ranger', text: '', position: 0 } },
    },
    spells: {
      list: {
        [RECORD]: {
          name: 'Cure Wounds',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: '',
          position: 0,
        },
      },
    },
    resources: {
      list: {
        [RECORD]: {
          name: 'Ammunition',
          max: 20,
          used: 0,
          rechargeShort: 0,
          rechargeLong: 0,
          rechargeNone: 0,
          position: 0,
        },
      },
    },
    ...overrides,
  };
}

export function fifthCharacterSheet(overrides: Record<string, unknown> = {}): Sheet {
  return parseSheet({
    id: FIFTH_CHARACTER_SHEET_ID,
    template_id: FIFTH_CHARACTER_TEMPLATE_ID,
    kind: 'character',
    name: 'Brenna',
    data: fifthCharacterSheetData(),
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
    ...overrides,
  });
}

export function fifthStatBlockSheetData(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    identity: { name: 'Gray Ooze', challengeRating: 2, proficiencyBonus: 2 },
    abilities: zeroAbilities(),
    saves: saves(),
    skills: skills(),
    exhaustion: { level: 0, penalty: 0 },
    passive: {
      choiceNone: 1,
      choiceAdvantage: 0,
      choiceDisadvantage: 0,
      perception: 10,
    },
    armor: {
      choiceNone: 1,
      choiceLight: 0,
      choiceMedium: 0,
      choiceHeavy: 0,
      choiceBarbarian: 0,
      choiceMonk: 0,
      choiceNatural: 0,
      base: 10,
      shield: 0,
      misc: 0,
      armorClass: 10,
    },
    experience: { xp: 450, zero: 1, inLair: 0 },
    hitPoints: { count: 2, size: 20, conBonus: 0, maximum: 9, current: 9, temp: 0 },
    initiative: { modifier: 0, score: 10 },
    speed: {
      walk: 10,
      walkTotal: 10,
      fly: 0,
      flyTotal: 0,
      swim: 0,
      swimTotal: 0,
      climb: 0,
      climbTotal: 0,
      burrow: 0,
      burrowTotal: 0,
    },
    details: { resistances: '', immunities: '', senses: '', languages: '' },
    traits: {
      [RECORD]: { name: 'Amorphous', text: 'It can squeeze through a narrow gap.', position: 0 },
    },
    actions: {
      [RECORD]: {
        abilityStrength: 1,
        abilityDexterity: 0,
        abilityConstitution: 0,
        abilityIntelligence: 0,
        abilityWisdom: 0,
        abilityCharisma: 0,
        proficient: 1,
        misc: 0,
        toHit: 3,
        damage: '1d6 + 1',
        notes: '',
        name: 'Pseudopod',
        position: 0,
      },
    },
    bonusActions: {
      [RECORD]: { name: 'Shift', text: 'Moves without provoking.', position: 0 },
    },
    reactions: {
      [RECORD]: { name: 'Split', text: 'Splits when struck.', position: 0 },
    },
    legendaryActions: {
      [RECORD]: { name: 'Engulf', text: 'Pulls a creature in.', position: 0 },
    },
    resources: {
      [RECORD]: {
        name: 'Recharge',
        max: 1,
        used: 0,
        recharge: 'Recharge 5-6',
        remaining: 1,
        position: 0,
      },
    },
    ...overrides,
  };
}

export function fifthStatBlockSheet(overrides: Record<string, unknown> = {}): Sheet {
  return parseSheet({
    id: FIFTH_STAT_BLOCK_SHEET_ID,
    template_id: FIFTH_STAT_BLOCK_TEMPLATE_ID,
    kind: 'npc',
    name: 'Gray Ooze',
    data: fifthStatBlockSheetData(),
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
    ...overrides,
  });
}

export const FIFTH_RECORD_ID = RECORD;
