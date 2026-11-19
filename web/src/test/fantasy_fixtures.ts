import type { Sheet } from '../api/models/sheet';

export const FANTASY_TEMPLATE_ID = 'fantasy_v1';

export const FANTASY_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd2a';

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
    combat: {
      label: 'Combat',
      children: {
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
  };
}

export function fantasySheetData(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  const { stats, modifiers, derived, combat, ...identity } = overrides;
  return {
    identity: {
      characterName: 'Aria',
      playerName: 'Sam',
      ancestry: 'Elf',
      origin: 'Wanderer',
      level: 3,
      proficiencyBonus: 2,
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
      passiveAwareness: 14,
      ...(derived as Record<string, unknown> | undefined),
    },
    combat: {
      hitPoints: 10,
      hitPointsMax: 20,
      hitPointsTemp: 0,
      ...(combat as Record<string, unknown> | undefined),
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
