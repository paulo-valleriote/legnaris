import { act } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ResetAction } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import { orderedRecords } from '../engine/row_identity';
import { message, messageWith } from '../l10n/strings';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { RecordTable } from '../sheet/preset/compose/record_table';
import { ResetBar } from '../sheet/preset/compose/reset_bar';
import { RollModeBar } from '../sheet/preset/compose/roll_mode_bar';
import { StatPills } from '../sheet/preset/compose/stat_pills';
import type { PresetConfig } from '../sheet/preset/compose/types';
import { DiceProvider } from '../sheet/dice_row';
import { PresetProvider } from '../sheet/preset/preset_context';
import {
  createPresetRuntime,
  presetConfig,
  presetSheet,
  presetTemplateDetail,
  renderPreset,
} from './preset_fixtures';

const RECORD_A = 'r0123456789abcdef0123456789abcdef';
const RECORD_B = 'r0123456789abcdef0123456789abcdee';
const RECORD_C = 'r0123456789abcdef0123456789abcde1';
const RECORD_D = 'r0123456789abcdef0123456789abcde2';

const REST: ResetAction = {
  id: 'rest',
  label: 'Rest',
  assignments: [{ target: 'charges.used', expression: '0' }],
};

function recordSheet(records: Record<string, Record<string, unknown>>) {
  return presetSheet({
    data: {
      identity: { name: 'Nova', subtitle: 'Scout' },
      vitals: { armor: 14, hit: 12, empty: '' },
      charges: { used: 1, limit: 4 },
      inventory: records,
    },
  });
}

function recordTable(columns?: readonly string[]) {
  return {
    kind: 'recordTable' as const,
    key: 'inventory',
    list: 'inventory',
    ...(columns === undefined ? {} : { columns }),
  };
}

function noDiceTemplate() {
  return parseTemplateDetail({
    id: 'compose_v1',
    name: 'No Dice',
    default: false,
    kinds: ['character'],
    fields: {
      inventory: {
        list: {
          label: 'Inventory',
          capacity: 4,
          columns: ['name'],
          shape: {
            name: { schema: { type: 'text', label: 'Item Name', is_editable: true, default: 0 } },
          },
        },
      },
    },
  });
}

function markedRowTemplate() {
  return parseTemplateDetail({
    id: 'compose_v1',
    name: 'Marked Row',
    default: false,
    kinds: ['character'],
    fields: {
      vitals: {
        label: 'Vitals',
        children: {
          armor: {
            schema: { type: 'number', label: 'Armor Class', is_editable: true, default: 10 },
          },
        },
      },
    },
    dice: { row: { 'vitals.armor': '1d20+4' }, not_d20: { row: ['vitals.armor'] } },
  });
}

function exclusiveTemplate() {
  return parseTemplateDetail({
    id: 'compose_v1',
    name: 'Exclusive Record',
    default: false,
    kinds: ['character'],
    fields: {
      inventory: {
        list: {
          label: 'Inventory',
          capacity: 4,
          columns: ['name'],
          shape: {
            name: { schema: { type: 'text', label: 'Item Name', is_editable: true, default: 0 } },
            equipped: {
              schema: {
                type: 'number',
                label: 'Equipped',
                is_editable: true,
                default: 0,
                exclusive_group: 'slot',
              },
            },
            stowed: {
              schema: {
                type: 'number',
                label: 'Stowed',
                is_editable: true,
                default: 0,
                exclusive_group: 'slot',
              },
            },
          },
        },
      },
    },
  });
}

function resetBar(actions: readonly string[]) {
  return { kind: 'resetBar' as const, key: 'resets', actions };
}

function rollModeBar() {
  return { kind: 'rollMode' as const, key: 'mode' };
}

function resetBinding(
  run: (action: ResetAction, requestId: string) => Promise<Record<string, unknown>>,
) {
  return {
    run,
    onApplied: vi.fn(),
    refresh: vi.fn(async () => undefined),
    requestId: () => 'request-1',
  };
}

function addName() {
  return messageWith('record.add', { list: 'Inventory' });
}

describe('the record table piece', () => {
  it('Happy path: chosen columns draw their original labels and stored values', () => {
    renderPreset(<RecordTable piece={recordTable(['name', 'uses'])} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    const row = screen.getByTestId('compose-record-row');
    expect(within(row).getByText('Item Name')).toBeInTheDocument();
    expect(within(row).getByText('Uses')).toBeInTheDocument();
    expect(within(row).getByRole('textbox', { name: 'Item Name' })).toHaveValue('Sword');
    expect(within(row).getByRole('textbox', { name: 'Uses' })).toHaveValue('2');
  });

  it('Happy path: the table names its list in a visible heading', () => {
    renderPreset(<RecordTable piece={recordTable(['name', 'uses'])} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    const table = screen.getByTestId('compose-record-table');
    const heading = within(table).getByRole('heading', { level: 2 });
    expect(heading).toHaveTextContent(table.getAttribute('aria-label') ?? '');
  });

  it('Edge: an empty list still shows its heading', () => {
    renderPreset(<RecordTable piece={recordTable()} />, { sheet: recordSheet({}) });

    expect(
      within(screen.getByTestId('compose-record-table')).getByRole('heading', { level: 2 }),
    ).toBeInTheDocument();
  });

  it('Happy path: columns share one grid template, text wider than numbers', () => {
    renderPreset(<RecordTable piece={recordTable(['name', 'uses'])} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    expect(
      screen.getByTestId('compose-record-table').style.getPropertyValue('--lg-record-template'),
    ).toBe('minmax(0, 2fr) minmax(6.5rem, 1fr)');
  });

  it('Happy path: one column header covers every Record in the run', () => {
    renderPreset(<RecordTable piece={recordTable(['name', 'uses'])} />, {
      sheet: recordSheet({
        [RECORD_A]: { name: 'Sword', uses: 2, position: 1 },
        [RECORD_B]: { name: 'Rope', uses: 1, position: 2 },
      }),
    });

    const headers = document.querySelectorAll('.lg-compose-record__header');
    expect(headers).toHaveLength(1);
    expect(within(headers[0] as HTMLElement).getByText('Item Name')).toBeInTheDocument();
    expect(screen.getAllByTestId('compose-record-row')).toHaveLength(2);
  });

  it('Happy path: the table subscribes to each cell path rather than the whole sheet', () => {
    const runtime = createPresetRuntime(
      presetTemplateDetail(),
      recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    );
    const spy = vi.spyOn(runtime.store, 'subscribePath');

    render(
      <PresetProvider {...runtime.preset}>
        <DiceProvider value={runtime.dice}>
          <RecordTable piece={recordTable(['name', 'uses'])} />
        </DiceProvider>
      </PresetProvider>,
    );

    expect(spy).toHaveBeenCalledWith(`inventory.${RECORD_A}.name`, expect.any(Function));
    expect(spy).toHaveBeenCalledWith(`inventory.${RECORD_A}.uses`, expect.any(Function));
  });

  it('Edge: a store without removeRecord draws no Remove control', () => {
    const runtime = createPresetRuntime(
      presetTemplateDetail(),
      recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    );

    render(
      <PresetProvider {...runtime.preset} store={{ ...runtime.store, removeRecord: undefined }}>
        <DiceProvider value={runtime.dice}>
          <RecordTable piece={recordTable()} />
        </DiceProvider>
      </PresetProvider>,
    );

    expect(
      screen.queryByRole('button', {
        name: messageWith('record.remove', { label: 'Inventory 1' }),
      }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: addName() })).toBeInTheDocument();
  });

  it('Edge: an exclusive column gets a wider track so its choice is not clipped', () => {
    renderPreset(<RecordTable piece={recordTable(['name', 'slot'])} />, {
      template: exclusiveTemplate(),
      sheet: recordSheet({
        [RECORD_A]: { name: 'Sword', equipped: 1, stowed: 0, position: 1 },
      }),
    });

    expect(
      screen.getByTestId('compose-record-table').style.getPropertyValue('--lg-record-template'),
    ).toBe('minmax(0, 2fr) minmax(8.5rem, 1.25fr)');
  });

  it('Happy path: an exclusive Record column selects one option through the store', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<RecordTable piece={recordTable(['name', 'slot'])} />, {
      template: exclusiveTemplate(),
      sheet: recordSheet({
        [RECORD_A]: { name: 'Sword', equipped: 1, stowed: 0, position: 1 },
      }),
    });

    const select = screen.getByRole('combobox', { name: 'Slot' });
    expect(select).toHaveValue(`inventory.${RECORD_A}.equipped`);

    await user.selectOptions(select, `inventory.${RECORD_A}.stowed`);

    expect(store.getPendingDelta()).toEqual({
      inventory: { [RECORD_A]: { equipped: 0, stowed: 1 } },
    });
  });

  it('Edge: an empty list draws the shared empty copy and keeps Add enabled', () => {
    renderPreset(<RecordTable piece={recordTable()} />, { sheet: recordSheet({}) });

    expect(screen.getByText(message('record.empty'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: addName() })).toBeEnabled();
  });

  it('Edge: a list at capacity disables Add with its visible reason', () => {
    renderPreset(<RecordTable piece={recordTable()} />, {
      sheet: recordSheet({
        [RECORD_A]: { name: 'Sword', uses: 2, position: 1 },
        [RECORD_B]: { name: 'Rope', uses: 1, position: 2 },
        [RECORD_C]: { name: 'Torch', uses: 3, position: 3 },
        [RECORD_D]: { name: 'Pick', uses: 4, position: 4 },
      }),
    });

    const add = screen.getByRole('button', { name: addName() });
    expect(add).toHaveAttribute('aria-disabled', 'true');
    expect(add).toHaveAccessibleDescription(
      messageWith('record.atCapacity', { count: 4, capacity: 4 }),
    );
  });

  it('Happy path: Add appends a Record through the edit store', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<RecordTable piece={recordTable()} />, {
      sheet: recordSheet({}),
    });

    await user.click(screen.getByRole('button', { name: addName() }));

    expect(orderedRecords('inventory', store.getState().data)).toHaveLength(1);
  });

  it('Edge: a locked sheet keeps text and number values as focusable read-only inputs', () => {
    renderPreset(<RecordTable piece={recordTable(['name', 'uses'])} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
      editing: false,
    });

    const name = screen.getByRole('textbox', { name: 'Item Name' });
    const uses = screen.getByRole('textbox', { name: 'Uses' });
    for (const input of [name, uses]) {
      expect(input).toHaveAttribute('readonly');
      expect(input).toHaveAttribute('aria-readonly', 'true');
      expect(input).not.toBeDisabled();
    }
    name.focus();
    expect(document.activeElement).toBe(name);
  });

  it('Edge: the Remove control is icon-only and keeps its accessible name', () => {
    renderPreset(<RecordTable piece={recordTable()} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    const name = messageWith('record.remove', { label: 'Inventory 1' });
    const remove = screen.getByRole('button', { name });
    expect(remove.textContent).toBe('');
    expect(screen.queryByText(name)).toBeNull();
  });

  it('Edge: removing the last Record empties the list', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<RecordTable piece={recordTable()} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    await user.click(
      screen.getByRole('button', { name: messageWith('record.remove', { label: 'Inventory 1' }) }),
    );

    expect(orderedRecords('inventory', store.getState().data)).toHaveLength(0);
    expect(screen.getByText(message('record.empty'))).toBeInTheDocument();
  });

  it('Edge: a Record with a missing column value draws an empty editable cell', () => {
    renderPreset(<RecordTable piece={recordTable(['name'])} />, {
      sheet: recordSheet({ [RECORD_A]: { uses: 2, position: 1 } }),
    });

    expect(screen.getByRole('textbox', { name: 'Item Name' })).toHaveValue('');
  });

  it('Edge: a chosen column the shape does not declare is a config error', () => {
    const config: PresetConfig = presetConfig({
      pieces: [recordTable(['name', 'ghost'])],
    });

    expect(configPathErrors(config, presetTemplateDetail())).toEqual(['inventory.ghost']);
  });

  it('Edge: an unknown chosen column draws no cell', () => {
    renderPreset(<RecordTable piece={recordTable(['ghost'])} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    expect(screen.queryByText('Ghost')).not.toBeInTheDocument();
    expect(
      document.querySelectorAll('[data-testid="compose-record-row"] .lg-compose-record__cell'),
    ).toHaveLength(0);
  });

  it('Happy path: a valid record table binds the list and its chosen columns', () => {
    const config: PresetConfig = presetConfig({
      pieces: [recordTable(['name', 'uses'])],
    });

    expect(configPathErrors(config, presetTemplateDetail())).toEqual([]);
  });

  it('Happy path: a list with a Row default dice draws a rollable Record Row', async () => {
    const user = userEvent.setup();
    renderPreset(<RecordTable piece={recordTable()} />, {
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    const roll = screen.getByRole('button', { name: /^Roll Inventory 1/ });
    roll.focus();
    await user.keyboard('{Enter}');

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Edge: a list without a Row default dice draws no roll control', () => {
    renderPreset(<RecordTable piece={recordTable()} />, {
      template: noDiceTemplate(),
      sheet: recordSheet({ [RECORD_A]: { name: 'Sword', uses: 2, position: 1 } }),
    });

    expect(screen.queryByRole('button', { name: /^Roll Inventory 1/ })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Item Name' })).toHaveValue('Sword');
  });
});

describe('the reset action bar piece', () => {
  it('Happy path: only the bound action ids draw buttons, in binding order', () => {
    renderPreset(<ResetBar piece={resetBar(['rest'])} />, {
      reset: resetBinding(vi.fn(async () => ({}))),
    });

    const group = screen.getByRole('group', { name: message('sheet.reset.group') });
    expect(
      within(group)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Rest']);
  });

  it('Edge: an unbound action id draws no button', () => {
    renderPreset(<ResetBar piece={resetBar(['ghost'])} />, {
      reset: resetBinding(vi.fn(async () => ({}))),
    });

    expect(
      within(screen.getByRole('group', { name: message('sheet.reset.group') })).queryAllByRole(
        'button',
      ),
    ).toHaveLength(0);
  });

  it('Edge: an unknown action id is a config error', () => {
    const config: PresetConfig = presetConfig({ pieces: [resetBar(['ghost'])] });

    expect(configPathErrors(config, presetTemplateDetail())).toEqual(['ghost']);
  });

  it('Edge: a pending draft disables the action and sends nothing', async () => {
    const user = userEvent.setup();
    const run = vi.fn(async () => ({}));
    const { store } = renderPreset(<ResetBar piece={resetBar(['rest'])} />, {
      reset: resetBinding(run),
    });

    act(() => store.setPath('vitals.hit', 3));

    const button = screen.getByRole('button', { name: 'Rest' });
    expect(button).toHaveAttribute('aria-disabled', 'true');
    expect(button).toHaveAccessibleDescription(message('sheet.reset.pending'));
    await user.click(button);

    expect(run).not.toHaveBeenCalled();
  });

  it('Happy path: a confirmed reset runs once, applies the cascade and announces it', async () => {
    const user = userEvent.setup();
    const cascade = { charges: { used: 0 } };
    const run = vi.fn(async () => cascade);
    const binding = resetBinding(run);
    renderPreset(<ResetBar piece={resetBar(['rest'])} />, { reset: binding });

    await user.click(screen.getByRole('button', { name: 'Rest' }));

    expect(run).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledWith(REST, 'request-1');
    expect(binding.onApplied).toHaveBeenCalledWith(REST, cascade);
    expect(screen.getByTestId('sheet-reset-notice')).toHaveTextContent(
      messageWith('sheet.reset.applied', { label: 'Rest' }),
    );
  });

  it('Error: a failed reset shows the failure copy and does not apply', async () => {
    const user = userEvent.setup();
    const run = vi.fn(async () => {
      throw new Error('network');
    });
    const binding = resetBinding(run);
    renderPreset(<ResetBar piece={resetBar(['rest'])} />, { reset: binding });

    await user.click(screen.getByRole('button', { name: 'Rest' }));

    expect(await screen.findByTestId('sheet-reset-error')).toHaveTextContent(
      message('api.error.unknown'),
    );
    expect(binding.onApplied).not.toHaveBeenCalled();
  });
});

describe('the roll mode bar piece', () => {
  it('Happy path: Normal is selected by default and every mode is a keyboard button', () => {
    renderPreset(<RollModeBar piece={rollModeBar()} />);

    const group = screen.getByRole('group', { name: message('preset.rollMode.group') });
    expect(
      within(group)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual([
      message('dice.mode.normal'),
      message('dice.mode.advantage'),
      message('dice.mode.disadvantage'),
    ]);
    expect(
      within(group).getByRole('button', { name: message('dice.mode.normal') }),
    ).toHaveAttribute('aria-pressed', 'true');
  });

  it('Happy path: a chosen Advantage applies to the next roll, then returns to Normal', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(
      <>
        <RollModeBar piece={rollModeBar()} />
        <StatPills
          piece={{ kind: 'statPills', key: 'vitals', pills: [{ path: 'vitals.armor' }] }}
        />
      </>,
    );

    const advantage = screen.getByRole('button', { name: message('dice.mode.advantage') });
    advantage.focus();
    await user.keyboard('{Enter}');
    expect(advantage).toHaveAttribute('aria-pressed', 'true');

    await user.click(screen.getByRole('button', { name: /Armor Class/ }));

    expect(document.querySelector('.lg-dice-panel')?.textContent).toContain(
      message('dice.mode.advantage'),
    );
    expect(screen.getByRole('button', { name: message('dice.mode.normal') })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a not_d20 Row keeps the chosen Advantage and rolls normal from the menu and keyboard', async () => {
    const user = userEvent.setup();
    renderPreset(
      <>
        <RollModeBar piece={rollModeBar()} />
        <StatPills
          piece={{ kind: 'statPills', key: 'vitals', pills: [{ path: 'vitals.armor' }] }}
        />
      </>,
      { template: markedRowTemplate() },
    );

    const advantage = screen.getByRole('button', { name: message('dice.mode.advantage') });
    await user.click(advantage);

    const roll = screen.getByRole('button', { name: /Armor Class/ });
    expect(roll).not.toHaveAttribute('aria-haspopup', 'dialog');
    fireEvent.contextMenu(roll);

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.querySelector('.lg-dice-panel')?.textContent ?? '').not.toContain(
      message('dice.mode.advantage'),
    );

    roll.focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(advantage).toHaveAttribute('aria-pressed', 'true');
  });

  it('Edge: the bar stores nothing on the Sheet', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<RollModeBar piece={rollModeBar()} />);

    await user.click(screen.getByRole('button', { name: message('dice.mode.disadvantage') }));

    expect(store.getPendingDelta()).toEqual({});
  });
});

describe('the config adapter with the new pieces', () => {
  it('Happy path: a record table, a reset bar and a roll mode bar render as pieces', () => {
    const preset = presetFromConfig(
      presetConfig({
        pieces: [rollModeBar(), recordTable(), resetBar(['rest'])],
      }),
    );

    renderPreset(
      <>
        {preset.sections.map(({ key, Component }) => (
          <Component key={key} />
        ))}
      </>,
      { sheet: recordSheet({}), reset: resetBinding(vi.fn(async () => ({}))) },
    );

    expect(screen.getByTestId('compose-record-table')).toBeInTheDocument();
    expect(screen.getByTestId('compose-roll-mode')).toBeInTheDocument();
    expect(screen.getByRole('group', { name: message('sheet.reset.group') })).toBeInTheDocument();
  });
});
