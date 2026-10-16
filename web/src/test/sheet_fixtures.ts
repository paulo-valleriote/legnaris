import type { Sheet } from '../api/models/sheet';

/** The template identifier the sheet-cycle suites create and open. */
export const TEMPLATE_ID = 'fantasy_v1';

/** The identifier of the sheet the open suites load. */
export const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';

/** A second identifier, so a foreign-id or cross-sheet case is exercisable. */
export const OTHER_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1e';

/** A synthetic field tree exercising a text leaf, a number, a derived leaf and an exclusive group. */
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

/** The decoded sheet data the fixtures seed. */
export function sheetData(): Record<string, unknown> {
  return { identity: { characterName: 'Aragorn', level: 3 }, derived: { total: 4 } };
}

/** A template-detail payload with the synthetic field tree. */
export function templateDetailPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return { id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: fieldTree(), ...overrides };
}

/** A template-detail payload whose field tree defines no sections. */
export function emptyTemplateDetailPayload(): Record<string, unknown> {
  return { id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} };
}

/** A sheet payload in the backend's snake_case wire shape. */
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

/** The decoded sheet model, for seeding a query cache directly. */
export const SHEET_MODEL: Sheet = {
  id: SHEET_ID,
  templateId: TEMPLATE_ID,
  name: 'Aragorn',
  data: sheetData(),
  createdAt: '2026-10-01T21:38:54.635Z',
  updatedAt: '2026-10-02T09:00:00.000Z',
};
