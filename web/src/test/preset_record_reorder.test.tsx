import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { messageWith } from '../l10n/strings';
import { DiceProvider } from '../sheet/dice_row';
import { RecordTable } from '../sheet/preset/compose/record_table';
import { PresetProvider } from '../sheet/preset/preset_context';
import { createDemoEditStore } from '../landing/demo_store';
import {
  FIFTH_RECORD_ID,
  fifthCharacterSheet,
  fifthCharacterSheetData,
  fifthCharacterTemplate,
} from './fifth_fixtures';
import { createPresetRuntime, renderPreset } from './preset_fixtures';

const PIECE = { kind: 'recordTable', key: 'inventory', list: 'inventory.items' } as const;

function rowLabel(position: number): string {
  return messageWith('record.rowLabel', { list: 'Items', position });
}

function moveUp(position: number): string {
  return messageWith('record.moveUp', { label: rowLabel(position) });
}

function moveDown(position: number): string {
  return messageWith('record.moveDown', { label: rowLabel(position) });
}

function oneRecordSheet() {
  const data = fifthCharacterSheetData();
  const inventory = data.inventory as { items: Record<string, unknown> };
  return fifthCharacterSheet({
    data: {
      ...data,
      inventory: { ...inventory, items: { [FIFTH_RECORD_ID]: inventory.items[FIFTH_RECORD_ID] } },
    },
  });
}

describe('record reordering in the preset table', () => {
  it('Edge: the first row cannot move up and the last cannot move down', () => {
    const template = fifthCharacterTemplate();
    renderPreset(<RecordTable piece={PIECE} />, {
      template,
      sheet: fifthCharacterSheet(),
    });

    const rows = screen.getAllByTestId('compose-record-row');
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByRole('button', { name: moveUp(1) })).toBeDisabled();
    expect(within(rows[0]).getByRole('button', { name: moveDown(1) })).toBeEnabled();
    expect(within(rows[1]).getByRole('button', { name: moveUp(2) })).toBeEnabled();
    expect(within(rows[1]).getByRole('button', { name: moveDown(2) })).toBeDisabled();
  });

  it('Edge: a list with a single record draws no move buttons', () => {
    renderPreset(<RecordTable piece={PIECE} />, {
      template: fifthCharacterTemplate(),
      sheet: oneRecordSheet(),
    });

    expect(screen.getAllByTestId('compose-record-row')).toHaveLength(1);
    expect(document.querySelectorAll('.lg-compose-record__move')).toHaveLength(0);
  });

  it('Happy path: a move lands focus on the usable button of the moved row', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<RecordTable piece={PIECE} />, {
      template: fifthCharacterTemplate(),
      sheet: fifthCharacterSheet(),
    });

    await user.click(screen.getByRole('button', { name: moveDown(1) }));

    const moved = document.querySelector(`[data-record-key="inventory.items.${FIFTH_RECORD_ID}"]`);
    const active = document.activeElement as HTMLElement;
    expect(moved?.contains(active)).toBe(true);
    expect(active).toHaveAccessibleName(moveUp(2));
    expect(active).toBeEnabled();
    expect(store.getPendingDelta()).toEqual({
      inventory: { items: { [FIFTH_RECORD_ID]: { position: 2 } } },
    });
  });

  it('Happy path: a move is announced with its new position', async () => {
    const user = userEvent.setup();
    renderPreset(<RecordTable piece={PIECE} />, {
      template: fifthCharacterTemplate(),
      sheet: fifthCharacterSheet(),
    });

    await user.click(screen.getByRole('button', { name: moveDown(1) }));

    expect(screen.getByRole('status')).toHaveTextContent(
      messageWith('record.moved', { position: 2 }),
    );
  });

  it('Edge: a store without moveRecord draws no reorder controls', () => {
    const template = fifthCharacterTemplate();
    const sheet = fifthCharacterSheet();
    const runtime = createPresetRuntime(template, sheet);
    const store = createDemoEditStore(template, sheet, () => new Map());

    render(
      <PresetProvider
        store={store}
        template={template}
        sheet={sheet}
        dice={runtime.dice}
        groupHeadingLevel={3}
      >
        <DiceProvider value={runtime.dice}>
          <RecordTable piece={PIECE} />
        </DiceProvider>
      </PresetProvider>,
    );

    expect(screen.getAllByTestId('compose-record-row')).toHaveLength(2);
    expect(document.querySelectorAll('.lg-compose-record__move')).toHaveLength(0);
  });
});
