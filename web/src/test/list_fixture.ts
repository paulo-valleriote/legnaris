import { parseTemplateDetail } from '../api/models/template';
import type { FieldNode } from '../api/models/template';
import { SHEET_ID } from './sheet_fixtures';

export const LIST_TEMPLATE_ID = 'records_test_v1';

export const LIST_PATH = 'inventory.items';

export const LIST_RECORD_A = 'r0123456789abcdef0123456789abcdef';

export const LIST_RECORD_B = 'r0123456789abcdef0123456789abcdee';

export const LIST_RECORD_C = 'r0123456789abcdef0123456789abcde1';

export const LIST_CAPACITY = 2;

function numberLeaf(label: string, extra: Record<string, unknown> = {}) {
  return { schema: { type: 'number', label, is_editable: true, default: 0, ...extra } };
}

export function listFieldsPayload(): Record<string, unknown> {
  return {
    level: numberLeaf('Level', { default: 1 }),
    inventory: {
      label: 'Inventory',
      children: {
        items: {
          list: {
            label: 'Items',
            columns: ['name', 'quantity', 'weight', 'active'],
            capacity: LIST_CAPACITY,
            dice: '1d20 + @.quantity',
            shape: {
              name: {
                schema: { type: 'text', label: 'Name', is_editable: true, default: 0 },
              },
              quantity: numberLeaf('Quantity', { default: 1 }),
              weight: numberLeaf('Weight', { default: 1 }),
              active: numberLeaf('Active', { flag: true, default: 0 }),
              optionA: numberLeaf('Option A', {
                exclusive_group: 'pick',
                option_label: 'A',
              }),
              optionB: numberLeaf('Option B', {
                exclusive_group: 'pick',
                option_label: 'B',
              }),
              derivedTotal: {
                schema: {
                  type: 'number',
                  label: 'Derived total',
                  is_editable: false,
                  default: 0,
                  formula: 'record_quantity * record_weight',
                  depends_on: [['quantity'], ['weight']],
                },
              },
              scaled: {
                schema: {
                  type: 'number',
                  label: 'Scaled',
                  is_editable: false,
                  default: 0,
                  formula: 'record_quantity * level',
                  depends_on: [['quantity'], ['level']],
                },
              },
            },
          },
        },
      },
    },
  };
}

export function listTemplatePayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: LIST_TEMPLATE_ID,
    name: 'Records Test',
    default: false,
    kinds: ['character', 'npc', 'enemy'],
    fields: listFieldsPayload(),
    ...overrides,
  };
}

export function listFields(): Record<string, FieldNode> {
  return parseTemplateDetail(listTemplatePayload()).fields;
}

export function listData(): Record<string, unknown> {
  return {
    level: 2,
    inventory: {
      items: {
        [LIST_RECORD_A]: {
          name: 'Sword',
          quantity: 2,
          weight: 3,
          active: 1,
          optionA: 1,
          optionB: 0,
          position: 1,
          derivedTotal: 6,
          scaled: 4,
        },
        [LIST_RECORD_B]: {
          name: 'Rope',
          quantity: 1,
          weight: 2,
          active: 0,
          optionA: 0,
          optionB: 1,
          position: 2,
          derivedTotal: 2,
          scaled: 2,
        },
      },
    },
  };
}

export function listDataWithRoom(): Record<string, unknown> {
  const data = listData() as { level: number; inventory: { items: Record<string, unknown> } };
  delete data.inventory.items[LIST_RECORD_B];
  return data;
}

export function emptyListData(): Record<string, unknown> {
  return { level: 2, inventory: { items: {} } };
}

export function listSheetPayload(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: SHEET_ID,
    template_id: LIST_TEMPLATE_ID,
    name: 'Quartermaster',
    data: listData(),
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
    ...overrides,
  };
}
