import type { Sheet } from '../api/models/sheet';

export const TEMPLATE_ID = 'plain_v1';

export const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';

export const OTHER_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1e';

export function fieldTree(): Record<string, unknown> {
  return {
    identity: {
      label: 'Identity',
      children: {
        characterName: { schema: { type: 'text', label: 'Name', is_editable: true, default: 0 } },
        level: { schema: { type: 'number', label: 'Level', is_editable: true, default: 1 } },
      },
    },
    derived: {
      label: 'Derived',
      children: {
        total: {
          schema: {
            type: 'number',
            label: 'Total',
            formula: 'level + 1',
            is_editable: false,
            default: 0,
          },
        },
      },
    },
    ability: {
      label: 'Ability',
      children: {
        stealth: {
          schema: {
            type: 'number',
            label: 'Stealth',
            is_editable: true,
            default: 0,
            exclusive_group: 'ability',
            option_label: 'Stealth',
          },
        },
        athletics: {
          schema: {
            type: 'number',
            label: 'Athletics',
            is_editable: true,
            default: 0,
            exclusive_group: 'ability',
            option_label: 'Athletics',
          },
        },
      },
    },
  };
}

export function sheetData(): Record<string, unknown> {
  return { identity: { characterName: 'Aragorn', level: 3 }, derived: { total: 4 } };
}

export function templateDetailPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: TEMPLATE_ID,
    name: 'Fantasy',
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: fieldTree(),
    ...overrides,
  };
}

export function emptyTemplateDetailPayload(): Record<string, unknown> {
  return {
    id: TEMPLATE_ID,
    name: 'Fantasy',
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  };
}

export function sheetPayload(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: SHEET_ID,
    template_id: TEMPLATE_ID,
    name: 'Aragorn',
    data: sheetData(),
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
    ...overrides,
  };
}

export const SHEET_MODEL: Sheet = {
  id: SHEET_ID,
  templateId: TEMPLATE_ID,
  name: 'Aragorn',
  data: sheetData(),
  dice: { section: {}, row: {} },
  createdAt: '2026-10-01T21:38:54.635Z',
  updatedAt: '2026-10-02T09:00:00.000Z',
};
