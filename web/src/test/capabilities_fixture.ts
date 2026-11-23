import { parseTemplateDetail } from '../api/models/template';
import type { FieldNode } from '../api/models/template';

export const CAPABILITIES_TEMPLATE_ID = 'capabilities_test_v1';

export const CAPABILITIES_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd2d';

export const CAPABILITIES_LIST_PATH = 'inventory.items';

export const CAPABILITIES_RECORD_ACTIVE = 'r0123456789abcdef0123456789abcdef';

export const CAPABILITIES_RECORD_IDLE = 'r0123456789abcdef0123456789abcdee';

export const CAPABILITIES_ACTION_ID = 'rest';

export const CAPABILITIES_ACTION_LABEL = 'Long rest';

export const CAPABILITIES_FIELD_DICE = '{stats.level}d{stats.proficiency} min 50';

export const CAPABILITIES_ROW_DICE = '1d20 + stats.level';

export const CAPABILITIES_INSIGHT_FORMULA =
  'min(stats_level * 2, lookup(stats_level, 1, 10, 5, 20))';

export const CAPABILITIES_KINDS = ['npc', 'enemy'] as const;

function numberLeaf(label: string, extra: Record<string, unknown> = {}) {
  return { schema: { type: 'number', label, is_editable: true, default: 0, ...extra } };
}

function derivedLeaf(label: string, formula: string, dependsOn: readonly unknown[]) {
  return {
    schema: {
      type: 'number',
      label,
      is_editable: false,
      default: 0,
      formula,
      depends_on: dependsOn,
    },
  };
}

function aggregateLeaf(label: string, field: string, op: 'sum' | 'count') {
  return {
    schema: {
      type: 'number',
      label,
      is_editable: false,
      default: 0,
      aggregate: { list: CAPABILITIES_LIST_PATH, field, op },
    },
  };
}

export function capabilitiesRecordShape(): Record<string, unknown> {
  return {
    name: {
      schema: { type: 'text', label: 'Name', is_editable: true, default: 0, max_length: 20 },
    },
    quantity: numberLeaf('Quantity', { default: 1 }),
    weight: numberLeaf('Weight', { default: 1 }),
    active: numberLeaf('Active', { flag: true, default: 0 }),
    optionA: numberLeaf('Option A', { exclusive_group: 'pick', option_label: 'A' }),
    optionB: numberLeaf('Option B', { exclusive_group: 'pick', option_label: 'B' }),
    derivedTotal: derivedLeaf('Derived total', 'record_quantity * record_weight', [
      ['quantity'],
      ['weight'],
    ]),
    scaled: derivedLeaf('Scaled', 'record_quantity * stats_level', [
      ['quantity'],
      ['stats', 'level'],
    ]),
  };
}

export function capabilitiesFieldsPayload(): Record<string, unknown> {
  return {
    stats: {
      label: 'Stats',
      children: {
        level: numberLeaf('Level', { default: 1 }),
        proficiency: numberLeaf('Proficiency', { default: 2 }),
        insight: derivedLeaf('Insight', CAPABILITIES_INSIGHT_FORMULA, [['stats', 'level']]),
      },
    },
    flags: {
      label: 'Flags',
      children: {
        inspired: numberLeaf('Inspired', { flag: true, default: 0 }),
      },
    },
    inventory: {
      label: 'Inventory',
      children: {
        items: {
          list: {
            label: 'Items',
            columns: ['name', 'quantity', 'weight', 'active'],
            capacity: 3,
            dice: '1d6',
            shape: capabilitiesRecordShape(),
          },
        },
      },
    },
    totals: {
      label: 'Totals',
      children: {
        inventoryValue: aggregateLeaf('Inventory value', 'derivedTotal', 'sum'),
        inventoryCount: aggregateLeaf('Inventory count', 'quantity', 'count'),
      },
    },
  };
}

export function capabilitiesResetActions(): readonly Record<string, unknown>[] {
  return [
    {
      id: CAPABILITIES_ACTION_ID,
      label: CAPABILITIES_ACTION_LABEL,
      assignments: [
        { target: 'stats.level', expression: 'max(1, stats_level - 1)' },
        {
          target: `${CAPABILITIES_LIST_PATH}.quantity`,
          expression: 'record_quantity + 1',
          filter: 'active',
        },
      ],
    },
  ];
}

export function capabilitiesTemplatePayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: CAPABILITIES_TEMPLATE_ID,
    name: 'Capabilities Test',
    default: false,
    kinds: [...CAPABILITIES_KINDS],
    fields: capabilitiesFieldsPayload(),
    dice: {
      section: { stats: CAPABILITIES_FIELD_DICE },
      row: { 'stats.insight': CAPABILITIES_ROW_DICE },
      not_d20: { row: ['stats.insight'] },
    },
    actions: capabilitiesResetActions(),
    ...overrides,
  };
}

export function capabilitiesFields(): Record<string, FieldNode> {
  return parseTemplateDetail(capabilitiesTemplatePayload()).fields;
}

export function capabilitiesData(): Record<string, unknown> {
  return {
    stats: { level: 3, proficiency: 2, insight: 6 },
    flags: { inspired: 1 },
    inventory: {
      items: {
        [CAPABILITIES_RECORD_ACTIVE]: {
          name: 'Torch',
          quantity: 2,
          weight: 2,
          active: 1,
          optionA: 1,
          optionB: 0,
          position: 1,
          derivedTotal: 4,
          scaled: 6,
        },
        [CAPABILITIES_RECORD_IDLE]: {
          name: 'Rope',
          quantity: 1,
          weight: 1,
          active: 0,
          optionA: 0,
          optionB: 1,
          position: 2,
          derivedTotal: 1,
          scaled: 3,
        },
      },
    },
    totals: { inventoryValue: 5, inventoryCount: 1 },
  };
}

export function capabilitiesSheetPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: CAPABILITIES_SHEET_ID,
    template_id: CAPABILITIES_TEMPLATE_ID,
    kind: 'npc',
    name: 'Ember Warden',
    data: capabilitiesData(),
    dice: { section: {}, row: {} },
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
    ...overrides,
  };
}

export function capabilitiesCatalog(): unknown[] {
  return [capabilitiesTemplatePayload()];
}

export function capabilitiesResetCascade(): Record<string, unknown> {
  return {
    stats: { level: 2, insight: 4 },
    inventory: {
      items: {
        [CAPABILITIES_RECORD_ACTIVE]: { quantity: 3, derivedTotal: 6, scaled: 6 },
        [CAPABILITIES_RECORD_IDLE]: { scaled: 2 },
      },
    },
    totals: { inventoryValue: 7, inventoryCount: 2 },
  };
}
