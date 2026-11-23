import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import { RECORD_ID_PATTERN } from '../engine/row_identity';
import { rollName } from './dice_names';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { fantasySheetPayload, fantasyTemplatePayload, FANTASY_SHEET_ID } from './fantasy_fixtures';
import {
  LIST_PATH,
  LIST_RECORD_A,
  LIST_RECORD_B,
  LIST_RECORD_C,
  emptyListData,
  listData,
  listDataWithRoom,
  listSheetPayload,
  listTemplatePayload,
} from './list_fixture';
import { createArrangeStub } from './arrange_stub';
import { CAMPAIGN_ID, baseRoutes, clientFor, ok, scripted } from './campaign_helpers';
import { renderApp } from './render';
import { SHEET_ID } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${SHEET_ID}`;
const GM = `/campaigns/${CAMPAIGN_ID}/sheets/${SHEET_ID}`;

function recordRows(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-row-kind="record"]'));
}

function addButton(): HTMLElement {
  return screen.getByRole('button', { name: messageWith('record.add', { list: 'Items' }) });
}

function liveRegion(): string {
  return screen.getByTestId('record-live').textContent ?? '';
}

async function open(
  options: {
    readonly data?: Record<string, unknown>;
    readonly patchOutcomes?: NonNullable<Parameters<typeof createArrangeStub>[0]>['patchOutcomes'];
    readonly sheetStore?: ReturnType<typeof createMemorySheetStore>;
  } = {},
) {
  const stub = createArrangeStub({
    template: listTemplatePayload(),
    data: options.data ?? listData(),
    ...(options.patchOutcomes === undefined ? {} : { patchOutcomes: options.patchOutcomes }),
  });
  const user = userEvent.setup();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    ...(options.sheetStore === undefined ? {} : { sheetStore: options.sheetStore }),
  });
  await screen.findByTestId('sheet-name');
  return { stub, user, view };
}

afterEach(cleanup);

describe('record list on the mounted sheet', () => {
  it('Happy path: the generic Edit form draws the list as a table of two records', async () => {
    await open();

    expect(screen.getByRole('heading', { name: 'Inventory' })).toBeInTheDocument();
    expect(recordRows()).toHaveLength(2);
    const first = recordRows()[0] as HTMLElement;
    expect(within(first).getByRole('textbox', { name: 'Name' })).toHaveValue('Sword');
    expect(within(first).getByRole('group', { name: /^Derived total(:| |$)/ })).toHaveTextContent(
      '6',
    );
  });

  it('Happy path: Add appends a row through the draft, focuses it and announces it', async () => {
    await open({ data: listDataWithRoom() });

    addButton().focus();
    await userEvent.keyboard('{Enter}');

    await waitFor(() => expect(recordRows()).toHaveLength(2));
    const added = recordRows()[1] as HTMLElement;
    expect(RECORD_ID_PATTERN.test(added.dataset.rowKey?.split('.').at(-1) ?? '')).toBe(true);
    expect(within(added).getByRole('textbox', { name: 'Name' })).toHaveFocus();
    expect(liveRegion()).toBe(messageWith('record.added', { count: 2 }));
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  });

  it('Edge: at capacity Add is disabled with a reason and no row is added', async () => {
    await open();

    const add = addButton();
    expect(add).toHaveAttribute('aria-disabled', 'true');
    expect(
      screen.getByText(messageWith('record.atCapacity', { count: 2, capacity: 2 })),
    ).toBeInTheDocument();
    await userEvent.click(add);
    expect(recordRows()).toHaveLength(2);
  });

  it('Happy path: removing an unsaved record drops it, focuses the next one and announces it', async () => {
    const { user } = await open();

    await user.click(
      within(recordRows()[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.remove', { label: 'Items 1' }),
      }),
    );

    expect(recordRows()).toHaveLength(1);
    expect(
      within(recordRows()[0] as HTMLElement).getByRole('textbox', { name: 'Name' }),
    ).toHaveValue('Rope');
    await waitFor(() =>
      expect(
        within(recordRows()[0] as HTMLElement).getByRole('textbox', { name: 'Name' }),
      ).toHaveFocus(),
    );
    expect(liveRegion()).toBe(messageWith('record.removed', { count: 1 }));
  });

  it('Happy path: removing the last record moves focus back to Add', async () => {
    const { user } = await open({ data: listDataWithRoom() });

    await user.click(
      within(recordRows()[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.remove', { label: 'Items 1' }),
      }),
    );

    await waitFor(() => expect(recordRows()).toHaveLength(0));
    await waitFor(() => expect(addButton()).toHaveFocus());
  });

  it('Happy path: F5 discard restores a removed record', async () => {
    const { user } = await open();

    await user.click(
      within(recordRows()[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.remove', { label: 'Items 1' }),
      }),
    );
    expect(recordRows()).toHaveLength(1);
    await user.keyboard('{F5}');
    await user.click(screen.getByRole('button', { name: message('sheet.refreshConfirm') }));

    await waitFor(() => expect(recordRows()).toHaveLength(2));
    expect(recordRows()[0]?.dataset.rowKey).toBe(`${LIST_PATH}.${LIST_RECORD_A}`);
    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
  });

  it('Happy path: moving down swaps the rows and announces the new position', async () => {
    const { user } = await open();

    await user.click(
      within(recordRows()[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveDown', { label: 'Items 1' }),
      }),
    );

    expect(recordRows().map((row) => row.dataset.rowKey)).toEqual([
      `${LIST_PATH}.${LIST_RECORD_B}`,
      `${LIST_PATH}.${LIST_RECORD_A}`,
    ]);
    expect(liveRegion()).toBe(messageWith('record.moved', { position: 2 }));
  });

  it('Happy path: a move keeps the keyboard focus inside the moved record', async () => {
    const { user } = await open();

    await user.click(
      within(recordRows()[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveDown', { label: 'Items 1' }),
      }),
    );

    await waitFor(() =>
      expect(recordRows()[1]).toContainElement(document.activeElement as HTMLElement),
    );
  });

  it('Edge: the first row cannot move up and the last cannot move down', async () => {
    await open();

    expect(
      within(recordRows()[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveUp', { label: 'Items 1' }),
      }),
    ).toBeDisabled();
    expect(
      within(recordRows()[1] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveDown', { label: 'Items 2' }),
      }),
    ).toBeDisabled();
  });

  it('Happy path: a Record edit previews through the same delta and marks the row', async () => {
    const fake = createFakeTransport(
      successOutcome(listSheetPayload()),
      successOutcome(listTemplatePayload()),
      successOutcome({}),
    );
    const user = userEvent.setup();
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByTestId('sheet-name');

    const quantity = within(recordRows()[0] as HTMLElement).getByRole('textbox', {
      name: 'Quantity',
    });
    await user.clear(quantity);
    await user.type(quantity, '3');
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));

    await waitFor(() =>
      expect(fake.requests.some((request) => request.url.endsWith('/preview'))).toBe(true),
    );
    const preview = fake.requests.find((request) => request.url.endsWith('/preview'));
    expect(JSON.parse(preview?.body ?? '{}')).toMatchObject({
      data: { inventory: { items: { [LIST_RECORD_A]: { quantity: 3 } } } },
    });
    expect(await screen.findByText(message('engine.modified'))).toBeInTheDocument();
  });

  it('Happy path: an exclusive choice inside a Record writes both members in one save', async () => {
    const { stub, user } = await open();

    await user.selectOptions(
      within(recordRows()[0] as HTMLElement).getByRole('combobox', { name: 'Pick' }),
      `${LIST_PATH}.${LIST_RECORD_A}.optionB`,
    );
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    await waitFor(() => expect(stub.patches()).toHaveLength(1));
    expect(JSON.parse(stub.patches()[0]?.body ?? '{}')).toMatchObject({
      data: {
        inventory: {
          items: {
            [LIST_RECORD_A]: { optionA: 0, optionB: 1 },
          },
        },
      },
    });
  });

  it('Happy path: a Record pins as its own Row and commits in the layout', async () => {
    const { stub, user } = await open();

    await user.click(screen.getByRole('button', { name: message('arrange.start') }));
    expect(
      screen.queryByRole('button', { name: messageWith('record.add', { list: 'Items' }) }),
    ).toBeNull();
    await user.click(
      screen.getByRole('button', {
        name: messageWith('arrange.pinLabel', { label: 'Items 1' }),
      }),
    );
    await user.click(screen.getByRole('button', { name: message('arrange.done') }));

    await waitFor(() => expect(stub.puts()).toHaveLength(1));
    expect(JSON.parse(stub.puts()[0]?.body ?? '{}')).toMatchObject({
      layout: { pinned: [`${LIST_PATH}.${LIST_RECORD_A}`] },
    });
  });

  it('Edge: an empty list shows the empty line and offers Add', async () => {
    await open({ data: emptyListData() });

    expect(recordRows()).toHaveLength(0);
    expect(screen.getByText(message('record.empty'))).toBeInTheDocument();
    expect(addButton()).toBeEnabled();
  });

  it('Edge: a 409 on save keeps the added record and the draft', async () => {
    const { user } = await open({
      data: listDataWithRoom(),
      patchOutcomes: [errorOutcome('CONFLICT', 'stale', 409)],
    });

    await user.click(addButton());
    await waitFor(() => expect(recordRows()).toHaveLength(2));
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    expect(
      await screen.findByTestId('sheet-save-error', undefined, { timeout: 8000 }),
    ).toHaveTextContent(message('sheet.error.saveConflict'));
    expect(recordRows()).toHaveLength(2);
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  }, 15000);

  it('Happy path: a restored local draft keeps an added record and a removal', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, {
      inventory: {
        items: {
          [LIST_RECORD_A]: null,
          [LIST_RECORD_C]: { position: 3, name: 'Bow', quantity: 4 },
        },
      },
    });
    await open({ sheetStore });

    await waitFor(() => expect(recordRows()).toHaveLength(2));
    expect(recordRows().map((row) => row.dataset.rowKey)).toEqual([
      `${LIST_PATH}.${LIST_RECORD_B}`,
      `${LIST_PATH}.${LIST_RECORD_C}`,
    ]);
    expect(
      within(recordRows()[1] as HTMLElement).getByRole('textbox', { name: 'Name' }),
    ).toHaveValue('Bow');
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  });

  it('Happy path: a Record roll takes advantage on Shift-click', async () => {
    const { user } = await open();

    const roll = within(recordRows()[0] as HTMLElement).getByRole('button', {
      name: rollName('Items 1'),
    });
    await user.keyboard('{Shift>}');
    await user.click(roll);
    await user.keyboard('{/Shift}');

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelectorAll('.lg-dice-panel__die')).toHaveLength(2);
    expect(document.querySelector('.lg-dice-panel')?.textContent).toContain(
      message('dice.mode.advantage'),
    );
  }, 15000);

  it('Edge: fantasy_v1 still draws no record table', async () => {
    const fake = createFakeTransport(
      successOutcome(fantasySheetPayload()),
      successOutcome(fantasyTemplatePayload()),
    );
    renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });

    await screen.findByTestId('sheet-name');
    expect(recordRows()).toHaveLength(0);
  });
});

describe('record list in the read-only campaign view', () => {
  function gmRoutes() {
    return [
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/templates/records_test_v1',
        respond: () => ok(listTemplatePayload()),
      },
      {
        method: 'GET',
        path: GM,
        respond: () => ok(listSheetPayload({ name: 'Quartermaster' })),
      },
    ];
  }

  it('Happy path: the GM view renders the same table with values and no controls', async () => {
    const script = scripted(gmRoutes());
    renderApp(GM, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('heading', { name: 'Quartermaster', level: 2 }),
    ).toBeInTheDocument();
    await waitFor(() => expect(recordRows()).toHaveLength(2));
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Add / })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Remove / })).not.toBeInTheDocument();
    expect(screen.getByText('Sword')).toBeInTheDocument();
    expect(screen.getByText('A')).toBeInTheDocument();
    const total = Array.from(document.querySelectorAll('.lg-record-cell__value--derived')).map(
      (cell) => cell.textContent,
    );
    expect(total).toEqual(['6', '4', '2', '2']);
  });

  it('Edge: an empty list in the GM view says so and offers no control', async () => {
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/templates/records_test_v1',
        respond: () => ok(listTemplatePayload()),
      },
      {
        method: 'GET',
        path: GM,
        respond: () => ok(listSheetPayload({ name: 'Quartermaster', data: emptyListData() })),
      },
    ]);
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Quartermaster', level: 2 });
    await waitFor(() => expect(screen.getByText(message('record.empty'))).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /^Add / })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Remove / })).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });
});
