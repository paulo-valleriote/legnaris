import { useState, useSyncExternalStore } from 'react';
import { recordsByList } from '../../../engine/row_identity';
import { SheetForm } from '../../../engine/sheet_form';
import type { RowActionsInfo } from '../../../engine/sheet_form';
import { message } from '../../../l10n/strings';
import { ChevronDownIcon } from '../../../ui/icons';
import { DICE_FRAMES, DiceEditLockContext, DiceRowActions } from '../../dice_row';
import { usePresetContext } from '../preset_context';
import './compose.css';

const noControl = () => null;

const rowActions = (info: RowActionsInfo) => <DiceRowActions {...info} />;

function FormulaRows() {
  const { template, store } = usePresetContext();
  const data = useSyncExternalStore(store.subscribe, () => store.getState().data);

  return (
    <SheetForm
      fields={template.fields}
      controlBuilder={noControl}
      exclusiveControlBuilder={noControl}
      records={recordsByList(template.fields, data)}
      rowActionsBuilder={rowActions}
      frames={DICE_FRAMES}
    />
  );
}

export function DiceFormulas() {
  const [open, setOpen] = useState(false);
  const { editing } = usePresetContext();

  return (
    <details
      className="lg-compose-fields lg-compose-fields--disclosure lg-compose-dice"
      data-testid="compose-dice-formulas"
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary className="lg-compose-fields__toggle">
        <ChevronDownIcon className="lg-compose-fields__chevron" />
        <span className="lg-compose-fields__title">{message('preset.diceFormulas.title')}</span>
      </summary>
      {open && (
        <DiceEditLockContext.Provider value={!editing}>
          <FormulaRows />
        </DiceEditLockContext.Provider>
      )}
    </details>
  );
}
