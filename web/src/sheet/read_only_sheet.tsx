import { useMemo } from 'react';
import type { Sheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { buildSections, rowKey } from '../engine/field_grouping';
import { isRowKey as isTemplateRowKey } from '../engine/row_identity';
import { seedFromSheet } from '../engine/sheet_defaults';
import { SheetForm } from '../engine/sheet_form';
import { fieldLabelFromFields } from './dice_label';
import { createDiceStore, type DiceCheck } from './dice_store';
import { DiceProvider, type DiceContextValue } from './dice_row';
import {
  checkExpression,
  checkSectionExpression,
  diceSectionRows,
  diceSchemaFromFields,
  templateDiceDefaults,
  templateDiceMarking,
} from './dice_view';
import { PresetPlayView } from './preset/play_view';
import { PresetProvider } from './preset/preset_context';
import { presetFor } from './preset/registry';
import { createRollingWindow, type SheetEditState, type SheetEditStore } from './sheet_edit';

function readOnlyStore(template: TemplateDetail, sheet: Sheet): SheetEditStore {
  const data = seedFromSheet(template.fields, sheet.data);
  const dice = sheet.dice ?? { section: {}, row: {} };
  const state: SheetEditState = {
    defaults: data,
    data,
    editedPaths: new Set<string>(),
    isSaving: false,
    justSaved: false,
    saveError: undefined,
    revision: 0,
    previewedRevision: undefined,
    savedDice: dice,
    dice,
    previewMarks: undefined,
    isPreviewing: false,
    previewError: undefined,
    saveWindow: createRollingWindow(() => 0, Number.POSITIVE_INFINITY),
    previewWindow: createRollingWindow(() => 0, Number.POSITIVE_INFINITY),
  };
  const nothing = () => undefined;
  const unsubscribe = () => undefined;

  return {
    getState: () => state,
    getPendingDelta: () => ({}),
    getPendingDice: () => undefined,
    subscribe: () => unsubscribe,
    subscribePath: () => unsubscribe,
    subscribeMarks: () => unsubscribe,
    setPath: nothing,
    setExclusive: nothing,
    setDice: nothing,
    applyReset: nothing,
    refresh: async () => undefined,
    save: async () => undefined,
    preview: async () => undefined,
    restore: async () => undefined,
    discard: nothing,
    dispose: nothing,
  };
}

function diceValueFor(template: TemplateDetail, store: SheetEditStore): DiceContextValue {
  const sections = buildSections(template.fields);
  const schema = diceSchemaFromFields(template.fields);
  const sectionRows = diceSectionRows(template.fields);
  const check: DiceCheck = (target, key, expression) =>
    target === 'row'
      ? checkExpression(expression, schema, key)
      : checkSectionExpression(expression, schema, sectionRows.get(key) ?? []);
  const diceStore = createDiceStore({ edit: store, check });

  return {
    store: diceStore,
    schema,
    check,
    defaults: templateDiceDefaults(template.dice, template.fields),
    rowKeys: new Set(sections.flatMap((section) => section.rows.map(rowKey))),
    sectionRows,
    isRowKey: (key) => isTemplateRowKey(template.fields, key),
    notD20: templateDiceMarking(template.dice, template.fields),
    labelForPath: fieldLabelFromFields(template.fields),
    savedValues: () => store.getState().defaults,
    draftValues: () => ({}),
  };
}

export function ReadOnlySheet({
  sheet,
  template,
}: {
  readonly sheet: Sheet;
  readonly template: TemplateDetail;
}) {
  const preset = useMemo(() => presetFor(template), [template]);
  const store = useMemo(() => readOnlyStore(template, sheet), [template, sheet]);
  const dice = useMemo(() => diceValueFor(template, store), [template, store]);

  if (preset === undefined) {
    return (
      <div className="lg-sheet lg-sheet--readonly" data-testid="readonly-generic">
        <SheetForm fields={template.fields} data={store.getState().data} />
      </div>
    );
  }

  return (
    <PresetProvider
      store={store}
      template={template}
      sheet={sheet}
      dice={dice}
      editing={false}
      readOnly
    >
      <DiceProvider value={dice}>
        <PresetPlayView preset={preset} />
      </DiceProvider>
    </PresetProvider>
  );
}
