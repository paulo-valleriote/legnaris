import type { Sheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import characterGolden from '../../../testdata/templates/fifth_character_v1.json';

export const FIFTH_DEMO_TEMPLATE_ID = 'fifth_character_v1';

export const FIFTH_DEMO_SHEET_ID = '00000000-0000-7000-8000-000000000421';

export const FIFTH_DEMO_TEMPLATE = parseTemplateDetail(characterGolden.data);

const ABILITY_KEYS = [
  'strength',
  'dexterity',
  'constitution',
  'intelligence',
  'wisdom',
  'charisma',
] as const;

type AbilityKey = (typeof ABILITY_KEYS)[number];

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

type SkillKey = (typeof SKILL_KEYS)[number];

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

function abilityData(): Record<string, unknown> {
  const scores: Readonly<Record<AbilityKey, number>> = {
    strength: 12,
    dexterity: 10,
    constitution: 14,
    intelligence: 11,
    wisdom: 16,
    charisma: 13,
  };
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      { score: scores[ability], mod: Math.floor((scores[ability] - 10) / 2) },
    ]),
  );
}

function saveData(): Record<string, unknown> {
  const totals: Readonly<Record<AbilityKey, number>> = {
    strength: 1,
    dexterity: 0,
    constitution: 2,
    intelligence: 0,
    wisdom: 5,
    charisma: 3,
  };
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      {
        prof: ability === 'wisdom' || ability === 'charisma' ? 1 : 0,
        misc: 0,
        total: totals[ability],
      },
    ]),
  );
}

function skillData(): Record<string, unknown> {
  const totals: Readonly<Record<SkillKey, number>> = {
    acrobatics: 0,
    animalHandling: 3,
    arcana: 0,
    athletics: 1,
    deception: 1,
    history: 0,
    insight: 5,
    intimidation: 1,
    investigation: 0,
    medicine: 5,
    nature: 0,
    perception: 3,
    performance: 1,
    persuasion: 3,
    religion: 2,
    sleightOfHand: 0,
    stealth: 0,
    survival: 3,
  };
  const ranks: Readonly<Partial<Record<SkillKey, number>>> = {
    insight: 1,
    medicine: 1,
    persuasion: 1,
    religion: 1,
  };
  return Object.fromEntries(
    SKILL_KEYS.map((skill) => [skill, { rank: ranks[skill] ?? 0, misc: 0, total: totals[skill] }]),
  );
}

function conditionData(): Record<string, number> {
  return Object.fromEntries(CONDITION_KEYS.map((condition) => [condition, 0]));
}

function spellcastingData(): Record<string, unknown> {
  const slots = Object.fromEntries(
    SLOT_LEVELS.map((level) => {
      const total = level === 1 ? 4 : level === 2 ? 2 : 0;
      const spent = level === 1 ? 1 : 0;
      return [`slot${level}`, { total, spent, remaining: total - spent }];
    }),
  );
  return {
    abilityStrength: 0,
    abilityDexterity: 0,
    abilityConstitution: 0,
    abilityIntelligence: 0,
    abilityWisdom: 1,
    abilityCharisma: 0,
    abilityMod: 3,
    attack: 5,
    saveDC: 13,
    preparedLimit: 6,
    ...slots,
    pact: { count: 0, level: 0, used: 0, remaining: 0 },
  };
}

function attackRecord(fields: Record<string, unknown>, position: number): Record<string, unknown> {
  return {
    abilityStrength: 0,
    abilityDexterity: 0,
    abilityConstitution: 0,
    abilityIntelligence: 0,
    abilityWisdom: 0,
    abilityCharisma: 0,
    damageBonus: 0,
    misc: 0,
    proficient: 0,
    ...fields,
    position,
  };
}

function inventoryRecord(
  fields: Record<string, unknown>,
  position: number,
): Record<string, unknown> {
  return { attuned: 0, lineWeight: 0, notes: '', ...fields, position };
}

function recordID(index: number): string {
  return `r${index.toString(16).padStart(32, '0')}`;
}

export const FIFTH_DEMO_SHEET: Sheet = {
  id: FIFTH_DEMO_SHEET_ID,
  templateId: FIFTH_DEMO_TEMPLATE_ID,
  kind: 'character',
  name: 'Ysolde Ashgrave',
  data: {
    identity: {
      name: 'Ysolde Ashgrave',
      class: 'Cleric',
      subclass: 'Life Domain',
      species: 'Human',
      background: 'Acolyte',
      level: 3,
      jackOfAllTrades: 0,
      proficiencyBonus: 2,
    },
    abilities: abilityData(),
    saves: saveData(),
    skills: skillData(),
    exhaustion: { level: 0, penalty: 0 },
    passive: { choiceNone: 1, choiceAdvantage: 0, choiceDisadvantage: 0, perception: 13 },
    armor: {
      choiceNone: 0,
      choiceLight: 0,
      choiceMedium: 1,
      choiceHeavy: 0,
      choiceBarbarian: 0,
      choiceMonk: 0,
      base: 13,
      shield: 1,
      misc: 0,
      armorClass: 15,
    },
    hitDice: {
      primary: { size: 8, count: 3, spent: 0, remaining: 3 },
      d6: { extra: 0, spent: 0, remaining: 0 },
      d8: { extra: 0, spent: 0, remaining: 0 },
      d10: { extra: 0, spent: 0, remaining: 0 },
      d12: { extra: 0, spent: 0, remaining: 0 },
    },
    spellcasting: spellcastingData(),
    combat: {
      initiative: { alert: 0, misc: 0, total: 0 },
      speed: { base: 30, total: 30 },
      armorStrengthRequirement: 0,
      hp: { base: 8, perLevel: 5, misc: 0, max: 24, current: 24, temp: 0 },
      carryingCapacity: 180,
    },
    status: {
      xp: 900,
      heroicInspiration: 0,
      conditions: conditionData(),
      deathSaves: { successes: 0, failures: 0 },
    },
    coins: { cp: 0, sp: 10, ep: 0, gp: 15, pp: 0 },
    attacks: {
      list: {
        [recordID(1)]: attackRecord(
          {
            abilityStrength: 1,
            damage: '1d6 + 1',
            damageBonus: 1,
            name: 'Mace',
            notes: 'Melee, 5 ft.',
            proficient: 1,
            toHit: 3,
          },
          0,
        ),
        [recordID(2)]: attackRecord(
          {
            abilityDexterity: 1,
            damage: '1d8',
            name: 'Light Crossbow',
            notes: 'Ranged, 80/320 ft.',
            proficient: 1,
            toHit: 2,
          },
          1,
        ),
      },
    },
    inventory: {
      items: {
        [recordID(1)]: inventoryRecord(
          {
            name: 'Chain Shirt',
            quantity: 1,
            weight: 10,
            lineWeight: 10,
            notes: 'Medium armor',
          },
          0,
        ),
        [recordID(2)]: inventoryRecord(
          { name: 'Shield', quantity: 1, weight: 6, lineWeight: 6, notes: '+2 Armor Class' },
          1,
        ),
        [recordID(3)]: inventoryRecord(
          {
            name: 'Holy Symbol',
            quantity: 1,
            weight: 1,
            lineWeight: 1,
            notes: 'Focus for cleric spells',
          },
          2,
        ),
        [recordID(4)]: inventoryRecord({ name: 'Mace', quantity: 1, weight: 4, lineWeight: 4 }, 3),
        [recordID(5)]: inventoryRecord(
          { name: 'Light Crossbow', quantity: 1, weight: 5, lineWeight: 5 },
          4,
        ),
        [recordID(6)]: inventoryRecord(
          {
            name: 'Crossbow Bolts',
            quantity: 20,
            weight: 0.05,
            lineWeight: 1,
            notes: 'Ammunition',
          },
          5,
        ),
        [recordID(7)]: inventoryRecord(
          { name: "Explorer's Pack", quantity: 1, weight: 30, lineWeight: 30 },
          6,
        ),
      },
      totalWeight: 57,
      attuned: 0,
    },
    features: {
      list: {
        [recordID(1)]: {
          name: 'Life Domain',
          source: 'Cleric 3',
          text: 'Domain spells: Bless, Cure Wounds, Lesser Restoration, Spiritual Weapon.',
          position: 0,
        },
        [recordID(2)]: {
          name: 'Channel Divinity',
          source: 'Cleric 2',
          text: 'Turn Undead or Preserve Life; one use returns on a short rest.',
          position: 1,
        },
        [recordID(3)]: {
          name: 'Disciple of Life',
          source: 'Life Domain 3',
          text: 'Healing spells restore extra hit points equal to 2 plus the spell level.',
          position: 2,
        },
      },
    },
    spells: {
      list: {
        [recordID(1)]: {
          name: 'Guidance',
          level: 0,
          prepared: 1,
          concentration: 1,
          ritual: 0,
          notes: 'Cantrip; adds 1d4 to one ability check.',
          position: 0,
        },
        [recordID(2)]: {
          name: 'Sacred Flame',
          level: 0,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Cantrip; Dexterity save or 1d8 radiant damage.',
          position: 1,
        },
        [recordID(3)]: {
          name: 'Thaumaturgy',
          level: 0,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Cantrip; minor divine omen.',
          position: 2,
        },
        [recordID(4)]: {
          name: 'Bless',
          level: 1,
          prepared: 1,
          concentration: 1,
          ritual: 0,
          notes: 'Three creatures add 1d4 to attack rolls and saves.',
          position: 3,
        },
        [recordID(5)]: {
          name: 'Cure Wounds',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Touch restores 2d8 + Wisdom modifier hit points.',
          position: 4,
        },
        [recordID(6)]: {
          name: 'Healing Word',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Bonus action; restores 2d4 + Wisdom modifier hit points at range.',
          position: 5,
        },
        [recordID(7)]: {
          name: 'Guiding Bolt',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: '4d6 radiant damage; the next attack against the target has Advantage.',
          position: 6,
        },
        [recordID(8)]: {
          name: 'Lesser Restoration',
          level: 2,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Ends one disease or condition on a creature.',
          position: 7,
        },
        [recordID(9)]: {
          name: 'Spiritual Weapon',
          level: 2,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Bonus action; a spectral weapon attacks for force damage.',
          position: 8,
        },
      },
    },
    resources: {
      list: {
        [recordID(1)]: {
          name: 'Channel Divinity',
          max: 2,
          used: 0,
          rechargeShort: 0,
          rechargeLong: 1,
          rechargeNone: 0,
          position: 0,
        },
      },
    },
  },
  dice: { section: {}, row: {} },
  createdAt: '2026-11-24T00:00:00.000Z',
  updatedAt: '2026-11-24T00:00:00.000Z',
};
