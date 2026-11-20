import { useState } from 'react';
import type { ReactNode } from 'react';
import { buildSections, rowKey } from '../engine/field_grouping';
import { createDiceStore } from '../sheet/dice_store';
import { DiceProvider } from '../sheet/dice_row';
import type { DiceContextValue } from '../sheet/dice_row';
import {
  checkExpression,
  checkSectionExpression,
  diceSchemaFromFields,
  diceSectionRows,
  templateDiceDefaults,
} from '../sheet/dice_view';
import type { PresetContextValue } from '../sheet/preset/preset_context';
import { PresetProvider } from '../sheet/preset/preset_context';
import { DEMO_SHEET, DEMO_TEMPLATE } from './demo_character';
import { createDemoEditStore } from './demo_store';

type DemoPresetValue = Omit<PresetContextValue, 'fields'>;

interface DemoRuntime {
  readonly preset: DemoPresetValue;
  readonly dice: DiceContextValue;
}

export function createDemoRuntime(): DemoRuntime {
  const store = createDemoEditStore(DEMO_TEMPLATE, DEMO_SHEET);
  const sections = buildSections(DEMO_TEMPLATE.fields);
  const schema = diceSchemaFromFields(DEMO_TEMPLATE.fields);
  const sectionRows = diceSectionRows(DEMO_TEMPLATE.fields);
  const check = (target: 'row' | 'section', key: string, expression: string) =>
    target === 'row'
      ? checkExpression(expression, schema, key)
      : checkSectionExpression(expression, schema, sectionRows.get(key) ?? []);
  const diceStore = createDiceStore({ edit: store, check });
  const dice: DiceContextValue = {
    store: diceStore,
    schema,
    check,
    defaults: templateDiceDefaults(DEMO_TEMPLATE.dice),
    rowKeys: new Set(sections.flatMap((section) => section.rows.map(rowKey))),
    sectionRows,
    savedValues: () => store.getState().data,
    draftValues: () => store.getPendingDelta(),
  };

  return {
    preset: { store, template: DEMO_TEMPLATE, sheet: DEMO_SHEET, dice },
    dice,
  };
}

export function DemoProvider({ children }: { readonly children: ReactNode }) {
  const [runtime] = useState(createDemoRuntime);

  return (
    <PresetProvider {...runtime.preset}>
      <DiceProvider value={runtime.dice}>{children}</DiceProvider>
    </PresetProvider>
  );
}
