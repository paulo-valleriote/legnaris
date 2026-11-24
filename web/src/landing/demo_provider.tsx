import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { Sheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
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
  templateDiceMarking,
} from '../sheet/dice_view';
import type { PresetContextValue } from '../sheet/preset/preset_context';
import { PresetProvider } from '../sheet/preset/preset_context';
import type { DemoDerivation } from './demo_store';
import { createDemoEditStore, NO_DERIVATION } from './demo_store';
import { MOMENT_SHEET, MOMENT_TEMPLATE } from './moment_demo';

type DemoPresetValue = Pick<PresetContextValue, 'store' | 'template' | 'sheet' | 'dice'>;

interface DemoRuntime {
  readonly preset: DemoPresetValue;
  readonly dice: DiceContextValue;
}

export function createDemoRuntime(
  template: TemplateDetail = MOMENT_TEMPLATE,
  sheet: Sheet = MOMENT_SHEET,
  derive: DemoDerivation = NO_DERIVATION,
): DemoRuntime {
  const store = createDemoEditStore(template, sheet, derive);
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

  return {
    preset: { store, template, sheet, dice },
    dice,
  };
}

export function DemoProvider({
  children,
  onInteract,
  template = MOMENT_TEMPLATE,
  sheet = MOMENT_SHEET,
  derive = NO_DERIVATION,
}: {
  readonly children: ReactNode;
  readonly onInteract?: () => void;
  readonly template?: TemplateDetail;
  readonly sheet?: Sheet;
  readonly derive?: DemoDerivation;
}) {
  const [runtime] = useState(() => createDemoRuntime(template, sheet, derive));
  const onInteractRef = useRef(onInteract);

  useEffect(() => {
    onInteractRef.current = onInteract;
  }, [onInteract]);

  useEffect(() => {
    const store = runtime.preset.store;
    let revision = store.getState().revision;
    return store.subscribe(() => {
      const next = store.getState().revision;
      if (next !== revision) {
        revision = next;
        onInteractRef.current?.();
      }
    });
  }, [runtime]);

  return (
    <PresetProvider {...runtime.preset}>
      <DiceProvider value={runtime.dice} onRoll={onInteract}>
        {children}
      </DiceProvider>
    </PresetProvider>
  );
}
