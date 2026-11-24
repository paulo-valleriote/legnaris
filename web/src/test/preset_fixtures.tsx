import type { ReactNode } from 'react';
import { render } from '@testing-library/react';
import { createApiClient } from '../api/client';
import type { Sheet } from '../api/models/sheet';
import { parseSheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import { buildSections, rowKey } from '../engine/field_grouping';
import { createMemorySheetStore } from '../library/sheet_store';
import { createDiceStore } from '../sheet/dice_store';
import type { DiceContextValue } from '../sheet/dice_row';
import { DiceProvider } from '../sheet/dice_row';
import {
  checkExpression,
  checkSectionExpression,
  diceSchemaFromFields,
  diceSectionRows,
  templateDiceDefaults,
  templateDiceMarking,
} from '../sheet/dice_view';
import type { PresetConfig } from '../sheet/preset/compose/types';
import type { PresetResetBinding } from '../sheet/preset/preset_context';
import { PresetProvider } from '../sheet/preset/preset_context';
import type { SheetEditStore } from '../sheet/sheet_edit';
import { createSheetEditStore } from '../sheet/sheet_edit';
import { createFakeTransport, successOutcome } from './fake_transport';

export const PRESET_TEMPLATE_ID = 'compose_v1';

export const PRESET_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd2d';

export function presetTemplateDetail(): TemplateDetail {
  return parseTemplateDetail({
    id: PRESET_TEMPLATE_ID,
    name: 'Compose Template',
    default: false,
    kinds: ['character'],
    fields: {
      identity: {
        label: 'Identity',
        children: {
          name: { schema: { type: 'text', label: 'Name', is_editable: true, default: 0 } },
          subtitle: {
            schema: { type: 'text', label: 'Subtitle', is_editable: true, default: 0 },
          },
        },
      },
      vitals: {
        label: 'Vitals',
        children: {
          armor: {
            schema: { type: 'number', label: 'Armor Class', is_editable: true, default: 10 },
          },
          hit: {
            schema: { type: 'number', label: 'Hit Points', is_editable: true, default: 12 },
          },
          empty: { schema: { type: 'text', label: 'Empty Stat', is_editable: true, default: 0 } },
        },
      },
      charges: {
        label: 'Charges',
        children: {
          used: {
            schema: { type: 'number', label: 'Charges Used', is_editable: true, default: 0 },
          },
          limit: {
            schema: { type: 'number', label: 'Charge Limit', is_editable: true, default: 4 },
          },
        },
      },
      inventory: {
        list: {
          label: 'Inventory',
          capacity: 4,
          columns: ['name', 'uses'],
          shape: {
            name: { schema: { type: 'text', label: 'Item Name', is_editable: true, default: 0 } },
            uses: { schema: { type: 'number', label: 'Uses', is_editable: true, default: 0 } },
          },
          dice: '1d6',
        },
      },
    },
    dice: {
      section: {},
      row: { 'vitals.armor': '1d20+4' },
    },
    actions: [
      { id: 'rest', label: 'Rest', assignments: [{ target: 'charges.used', expression: '0' }] },
    ],
  });
}

export function presetSheet(overrides: Record<string, unknown> = {}): Sheet {
  return parseSheet({
    id: PRESET_SHEET_ID,
    template_id: PRESET_TEMPLATE_ID,
    name: 'Sheet Nova',
    data: {
      identity: { name: 'Nova', subtitle: 'Scout' },
      vitals: { armor: 14, hit: 12, empty: '' },
      charges: { used: 1, limit: 4 },
    },
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
    ...overrides,
  });
}

export function presetConfig(overrides: Partial<PresetConfig> = {}): PresetConfig {
  return {
    templateId: PRESET_TEMPLATE_ID,
    header: { name: 'identity.name', subtitle: 'identity.subtitle' },
    pieces: [
      {
        kind: 'statPills',
        key: 'vitals',
        pills: [{ path: 'vitals.armor' }, { path: 'vitals.hit' }],
      },
      { kind: 'pips', key: 'charges', count: 'charges.used', max: 'charges.limit' },
    ],
    ...overrides,
  };
}

function presetClient() {
  const fake = createFakeTransport(successOutcome({}));
  return createApiClient({ baseUrl: 'http://api.test', transport: fake.transport });
}

export interface PresetRuntime {
  readonly template: TemplateDetail;
  readonly sheet: Sheet;
  readonly store: SheetEditStore;
  readonly dice: DiceContextValue;
  readonly preset: {
    readonly store: SheetEditStore;
    readonly template: TemplateDetail;
    readonly sheet: Sheet;
    readonly dice: DiceContextValue;
  };
}

export function createPresetRuntime(
  template: TemplateDetail = presetTemplateDetail(),
  sheet: Sheet = presetSheet(),
): PresetRuntime {
  const store = createSheetEditStore(
    { template, sheet },
    { client: presetClient(), store: createMemorySheetStore() },
  );
  const sections = buildSections(template.fields);
  const schema = diceSchemaFromFields(template.fields);
  const sectionRows = diceSectionRows(template.fields);
  const check = (target: 'row' | 'section', key: string, expression: string) =>
    target === 'row'
      ? checkExpression(expression, schema, key)
      : checkSectionExpression(expression, schema, sectionRows.get(key) ?? []);
  const diceStore = createDiceStore({ edit: store, check });
  const dice: DiceContextValue = {
    store: diceStore,
    schema,
    check,
    defaults: templateDiceDefaults(template.dice, template.fields),
    rowKeys: new Set(sections.flatMap((section) => section.rows.map(rowKey))),
    sectionRows,
    notD20: templateDiceMarking(template.dice, template.fields),
    savedValues: () => store.getState().data,
    draftValues: () => store.getPendingDelta(),
  };
  return { template, sheet, store, dice, preset: { store, template, sheet, dice } };
}

export interface PresetRenderOptions {
  readonly template?: TemplateDetail;
  readonly sheet?: Sheet;
  readonly reset?: PresetResetBinding;
}

export function renderPreset(ui: ReactNode, options: PresetRenderOptions = {}) {
  const runtime = createPresetRuntime(options.template, options.sheet);
  const result = render(
    <PresetProvider {...runtime.preset} reset={options.reset}>
      <DiceProvider value={runtime.dice}>{ui}</DiceProvider>
    </PresetProvider>,
  );
  return { ...result, ...runtime };
}
