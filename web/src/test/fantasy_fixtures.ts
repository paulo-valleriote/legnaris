import type { Sheet } from '../api/models/sheet';

export const FANTASY_TEMPLATE_ID = 'fantasy_v1';

export const FANTASY_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd2a';

const FIXTURE_ABILITIES = [
  { key: 'strength', label: 'Strength' },
  { key: 'dexterity', label: 'Dexterity' },
  { key: 'constitution', label: 'Constitution' },
  { key: 'intelligence', label: 'Intelligence' },
  { key: 'wisdom', label: 'Wisdom' },
  { key: 'charisma', label: 'Charisma' },
] as const;

const FIXTURE_SKILLS = [
  { key: 'athletics', label: 'Athletics' },
  { key: 'acrobatics', label: 'Acrobatics' },
  { key: 'stealth', label: 'Stealth' },
  { key: 'perception', label: 'Perception' },
  { key: 'insight', label: 'Insight' },
  { key: 'survival', label: 'Survival' },
  { key: 'persuasion', label: 'Persuasion' },
  { key: 'deception', label: 'Deception' },
  { key: 'intimidation', label: 'Intimidation' },
  { key: 'knowledge', label: 'Knowledge' },
  { key: 'analysis', label: 'Analysis' },
  { key: 'craft', label: 'Craft' },
] as const;

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function savesFieldGroup(): Record<string, unknown> {
  return Object.fromEntries(
    FIXTURE_ABILITIES.map((ability) => [
      ability.key,
      {
        schema: {
          type: 'number',
          label: `${ability.label} save`,
          is_editable: false,
          default: 0,
        },
      },
    ]),
  );
}

function skillsFieldGroup(): Record<string, unknown> {
  return Object.fromEntries(
    FIXTURE_SKILLS.map((skill) => [
      skill.key,
      {
        children: {
          bonus: {
            schema: {
              type: 'number',
              label: `${skill.label} bonus`,
              is_editable: true,
              default: 0,
              min: -10,
              max: 20,
              column_label: 'Bonus',
            },
          },
          total: {
            schema: {
              type: 'number',
              label: `${skill.label} total`,
              is_editable: false,
              default: 0,
              column_label: 'Total',
            },
          },
        },
        columns: ['bonus', 'total'],
      },
    ]),
  );
}

function configFieldGroup(): Record<string, unknown> {
  return Object.fromEntries([
    ...FIXTURE_ABILITIES.map((ability) => [
      `save${capitalize(ability.key)}Prof`,
      {
        schema: {
          type: 'number',
          label: `${ability.label} save proficiency`,
          is_editable: true,
          default: 0,
          min: 0,
          max: 1,
        },
      },
    ]),
    ...FIXTURE_SKILLS.map((skill) => [
      `skill${capitalize(skill.key)}Prof`,
      {
        schema: {
          type: 'number',
          label: `${skill.label} proficiency`,
          is_editable: true,
          default: 0,
          min: 0,
          max: 1,
        },
      },
    ]),
  ]);
}

export function fantasyFieldTree(): Record<string, unknown> {
  return {
    identity: {
      label: 'Identity',
      children: {
        characterName: {
          schema: {
            type: 'text',
            label: 'Character name',
            is_editable: true,
            default: 0,
            max_length: 60,
          },
        },
        playerName: {
          schema: { type: 'text', label: 'Player name', is_editable: true, default: 0 },
        },
        ancestry: {
          schema: {
            type: 'text',
            label: 'Ancestry',
            is_editable: true,
            default: 0,
            max_length: 40,
          },
        },
        origin: {
          schema: {
            type: 'text',
            label: 'Origin',
            is_editable: true,
            default: 0,
            max_length: 40,
          },
        },
        level: {
          schema: {
            type: 'number',
            label: 'Level',
            is_editable: true,
            default: 1,
            min: 1,
            max: 20,
          },
        },
        proficiencyBonus: {
          schema: {
            type: 'number',
            label: 'Proficiency bonus',
            is_editable: true,
            default: 2,
            min: 2,
            max: 6,
          },
        },
        xp: {
          schema: {
            type: 'number',
            label: 'Experience points',
            is_editable: true,
            default: 0,
            min: 0,
            max: 355000,
          },
        },
      },
    },
    stats: {
      label: 'Stats',
      children: {
        strength: {
          schema: {
            type: 'number',
            label: 'Strength',
            is_editable: true,
            default: 10,
            min: 3,
            max: 20,
          },
        },
        dexterity: {
          schema: {
            type: 'number',
            label: 'Dexterity',
            is_editable: true,
            default: 10,
            min: 3,
            max: 20,
          },
        },
        constitution: {
          schema: {
            type: 'number',
            label: 'Constitution',
            is_editable: true,
            default: 10,
            min: 3,
            max: 20,
          },
        },
        intelligence: {
          schema: {
            type: 'number',
            label: 'Intelligence',
            is_editable: true,
            default: 10,
            min: 3,
            max: 20,
          },
        },
        wisdom: {
          schema: {
            type: 'number',
            label: 'Wisdom',
            is_editable: true,
            default: 10,
            min: 3,
            max: 20,
          },
        },
        charisma: {
          schema: {
            type: 'number',
            label: 'Charisma',
            is_editable: true,
            default: 10,
            min: 3,
            max: 20,
          },
        },
      },
    },
    modifiers: {
      label: 'Modifiers',
      children: {
        strengthMod: {
          schema: {
            type: 'number',
            label: 'Strength modifier',
            is_editable: false,
            default: 0,
          },
        },
        dexterityMod: {
          schema: {
            type: 'number',
            label: 'Dexterity modifier',
            is_editable: false,
            default: 0,
          },
        },
        constitutionMod: {
          schema: {
            type: 'number',
            label: 'Constitution modifier',
            is_editable: false,
            default: 0,
          },
        },
        intelligenceMod: {
          schema: {
            type: 'number',
            label: 'Intelligence modifier',
            is_editable: false,
            default: 0,
          },
        },
        wisdomMod: {
          schema: {
            type: 'number',
            label: 'Wisdom modifier',
            is_editable: false,
            default: 0,
          },
        },
        charismaMod: {
          schema: {
            type: 'number',
            label: 'Charisma modifier',
            is_editable: false,
            default: 0,
          },
        },
      },
    },
    derived: {
      label: 'Derived',
      children: {
        armorClass: {
          schema: {
            type: 'number',
            label: 'Armor class',
            is_editable: false,
            default: 10,
          },
        },
        passiveAwareness: {
          schema: {
            type: 'number',
            label: 'Passive awareness',
            is_editable: false,
            default: 10,
          },
        },
      },
    },
    config: {
      label: 'Template settings',
      children: {
        ...configFieldGroup(),
        acUsesDex: {
          schema: {
            type: 'number',
            label: 'Armor class uses dexterity',
            is_editable: true,
            default: 1,
            min: 0,
            max: 1,
            exclusive_group: 'armorClassAbility',
            option_label: 'Dexterity',
          },
        },
        acUsesCon: {
          schema: {
            type: 'number',
            label: 'Armor class uses constitution',
            is_editable: true,
            default: 0,
            min: 0,
            max: 1,
            exclusive_group: 'armorClassAbility',
            option_label: 'Constitution',
          },
        },
      },
    },
    combat: {
      label: 'Combat',
      children: {
        baseArmorClass: {
          schema: {
            type: 'number',
            label: 'Base armor class',
            is_editable: true,
            default: 10,
            min: 1,
            max: 30,
          },
        },
        initiative: {
          schema: {
            type: 'number',
            label: 'Initiative',
            is_editable: true,
            default: 0,
            min: -5,
            max: 20,
          },
        },
        speed: {
          schema: {
            type: 'number',
            label: 'Speed',
            is_editable: true,
            default: 30,
            min: 0,
            max: 120,
          },
        },
        hitPoints: {
          schema: {
            type: 'number',
            label: 'Current hit points',
            is_editable: true,
            default: 10,
            min: 0,
            max: 500,
          },
        },
        hitPointsMax: {
          schema: {
            type: 'number',
            label: 'Hit points (maximum)',
            is_editable: true,
            default: 10,
            min: 1,
            max: 500,
          },
        },
        hitPointsTemp: {
          schema: {
            type: 'number',
            label: 'Temporary hit points',
            is_editable: true,
            default: 0,
            min: 0,
            max: 100,
          },
        },
      },
    },
    saves: {
      label: 'Saves',
      children: savesFieldGroup(),
    },
    skills: {
      label: 'Skills',
      children: skillsFieldGroup(),
    },
  };
}

function savesData(): Record<string, unknown> {
  return {
    strength: 3,
    dexterity: 4,
    constitution: 1,
    intelligence: 0,
    wisdom: -1,
    charisma: 1,
  };
}

function skillsData(): Record<string, unknown> {
  return {
    athletics: { bonus: 0, total: 5 },
    acrobatics: { bonus: 0, total: 2 },
    stealth: { bonus: 1, total: 3 },
    perception: { bonus: 2, total: 1 },
    insight: { bonus: 0, total: -1 },
    survival: { bonus: 0, total: -1 },
    persuasion: { bonus: 0, total: 1 },
    deception: { bonus: 0, total: 1 },
    intimidation: { bonus: 0, total: 1 },
    knowledge: { bonus: 0, total: 0 },
    analysis: { bonus: 0, total: 0 },
    craft: { bonus: 0, total: 0 },
  };
}

function configData(): Record<string, unknown> {
  return {
    ...Object.fromEntries(
      FIXTURE_ABILITIES.map((ability) => [`save${capitalize(ability.key)}Prof`, 0]),
    ),
    ...Object.fromEntries(FIXTURE_SKILLS.map((skill) => [`skill${capitalize(skill.key)}Prof`, 0])),
    saveDexterityProf: 1,
    skillAthleticsProf: 1,
  };
}

export function fantasySheetData(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  const { stats, modifiers, derived, combat, saves, skills, config, ...identity } = overrides;
  return {
    identity: {
      characterName: 'Aria',
      playerName: 'Sam',
      ancestry: 'Elf',
      origin: 'Wanderer',
      level: 3,
      proficiencyBonus: 2,
      xp: 1400,
      ...identity,
    },
    stats: {
      strength: 16,
      dexterity: 14,
      constitution: 12,
      intelligence: 10,
      wisdom: 8,
      charisma: 13,
      ...(stats as Record<string, unknown> | undefined),
    },
    modifiers: {
      strengthMod: 3,
      dexterityMod: 2,
      constitutionMod: 1,
      intelligenceMod: 0,
      wisdomMod: -1,
      charismaMod: 1,
      ...(modifiers as Record<string, unknown> | undefined),
    },
    derived: {
      armorClass: 12,
      passiveAwareness: 14,
      ...(derived as Record<string, unknown> | undefined),
    },
    combat: {
      baseArmorClass: 10,
      initiative: 3,
      speed: 30,
      hitPoints: 10,
      hitPointsMax: 20,
      hitPointsTemp: 0,
      ...(combat as Record<string, unknown> | undefined),
    },
    saves: { ...savesData(), ...(saves as Record<string, unknown> | undefined) },
    skills: { ...skillsData(), ...(skills as Record<string, unknown> | undefined) },
    config: {
      acUsesDex: 1,
      acUsesCon: 0,
      ...configData(),
      ...(config as Record<string, unknown> | undefined),
    },
  };
}

export function fantasyTemplatePayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: FANTASY_TEMPLATE_ID,
    name: 'Fantasy Template',
    default: true,
    fields: fantasyFieldTree(),
    dice: {
      section: {
        skills: '1d20 + @.total',
        saves: '1d20 + @',
        attacks: '1d20 + @.bonus',
      },
      row: {
        'combat.initiative': '1d20 + @',
        'stats.strength': '1d20 + modifiers.strengthMod',
        'stats.dexterity': '1d20 + modifiers.dexterityMod',
        'stats.constitution': '1d20 + modifiers.constitutionMod',
        'stats.intelligence': '1d20 + modifiers.intelligenceMod',
        'stats.wisdom': '1d20 + modifiers.wisdomMod',
        'stats.charisma': '1d20 + modifiers.charismaMod',
      },
    },
    ...overrides,
  };
}

export function fantasySheetPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: FANTASY_SHEET_ID,
    template_id: FANTASY_TEMPLATE_ID,
    name: 'Aragorn',
    data: fantasySheetData(),
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
    ...overrides,
  };
}

export const FANTASY_SHEET_MODEL: Sheet = {
  id: FANTASY_SHEET_ID,
  templateId: FANTASY_TEMPLATE_ID,
  name: 'Aragorn',
  data: fantasySheetData(),
  dice: { section: {}, row: {} },
  createdAt: '2026-10-01T21:38:54.635Z',
  updatedAt: '2026-10-02T09:00:00.000Z',
};
