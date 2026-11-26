import { readFileSync } from 'node:fs';
import path from 'node:path';

const golden = JSON.parse(
  readFileSync(
    path.resolve(process.cwd(), '../testdata/templates/fifth_character_v1.json'),
    'utf8',
  ),
) as { data: Record<string, unknown> };

const RECORD_A = 'r0123456789abcdef0123456789abcdef';
const RECORD_B = 'r0123456789abcdef0123456789abcdee';

const ABILITIES = [
  'strength',
  'dexterity',
  'constitution',
  'intelligence',
  'wisdom',
  'charisma',
] as const;

const SKILLS = [
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

const CONDITIONS = [
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

const abilities = Object.fromEntries(ABILITIES.map((key) => [key, { score: 14, mod: 2 }]));
const saves = Object.fromEntries(ABILITIES.map((key) => [key, { prof: 0, misc: 0, total: 2 }]));
const skills = Object.fromEntries(SKILLS.map((key) => [key, { rank: 0, misc: 0, total: 2 }]));
const conditions = Object.fromEntries(CONDITIONS.map((key) => [key, 0]));
const slots = Object.fromEntries(
  [1, 2, 3, 4, 5, 6, 7, 8, 9].map((level) => [
    `slot${level}`,
    { total: 2, spent: 0, remaining: 2 },
  ]),
);

export const FIFTH_TEMPLATE_ID = 'fifth_character_v1';
export const FIFTH_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd90';

export const FIFTH_TEMPLATE = golden.data;

export const FIFTH_SHEET = {
  id: FIFTH_SHEET_ID,
  template_id: FIFTH_TEMPLATE_ID,
  kind: 'character',
  name: 'Brenna',
  data: {
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
    abilities,
    saves,
    skills,
    exhaustion: { level: 0, penalty: 0 },
    passive: { choiceNone: 1, choiceAdvantage: 0, choiceDisadvantage: 0, perception: 12 },
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
      armorClass: 12,
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
      ...slots,
      pact: { count: 1, level: 1, used: 0, remaining: 1 },
    },
    combat: {
      initiative: { alert: 0, misc: 0, total: 2 },
      speed: { base: 30, total: 30 },
      armorStrengthRequirement: 0,
      hp: { base: 36, perLevel: 8, misc: 0, max: 40, current: 28, temp: 4 },
      carryingCapacity: 150,
    },
    status: {
      xp: 1400,
      heroicInspiration: 0,
      conditions,
      deathSaves: { successes: 0, failures: 0 },
    },
    coins: { cp: 0, sp: 0, ep: 0, gp: 12, pp: 0 },
    attacks: {
      list: {
        [RECORD_A]: {
          abilityStrength: 1,
          abilityDexterity: 0,
          abilityConstitution: 0,
          abilityIntelligence: 0,
          abilityWisdom: 0,
          abilityCharisma: 0,
          proficient: 1,
          misc: 0,
          toHit: 5,
          damageBonus: 0,
          name: 'Longbow',
          damage: '1d8 + 3',
          notes: '',
          position: 0,
        },
      },
    },
    inventory: {
      items: {
        [RECORD_A]: {
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
      list: { [RECORD_A]: { name: 'Favored Enemy', source: 'Ranger', text: '', position: 0 } },
    },
    spells: {
      list: {
        [RECORD_A]: {
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
        [RECORD_A]: {
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
  },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};
