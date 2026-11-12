import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { LONG_PRESS_MS } from '../sheet/dice_row';
import { renderApp } from './render';
import { arrangeTemplatePayload, createArrangeStub } from './arrange_stub';
import { SHEET_ID } from './sheet_fixtures';

const SHEET_URL = `/sheets/${SHEET_ID}`;
const BASE_URL = 'http://api.test';

const DICE_TEMPLATE = {
  ...arrangeTemplatePayload(),
  dice: { section: {}, row: { 'alpha.one': '1d20+alpha.two' } },
};
const DATA = { alpha: { one: 5, two: 3, three: 0 } };

async function open(options: Parameters<typeof createArrangeStub>[0] = {}) {
  const stub = createArrangeStub({ template: DICE_TEMPLATE, data: DATA, ...options });
  const user = userEvent.setup();
  renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { stub, user };
}

const rollButton = (label: string) =>
  screen.queryByRole('button', { name: messageWith('dice.roll', { label }) });
const editButton = (label: string) =>
  screen.queryByRole('button', { name: messageWith('dice.edit', { label }) });
const sectionEditButton = (label: string) =>
  screen.queryByRole('button', { name: messageWith('dice.editSection', { label }) });
const editorDialog = (label: string) =>
  screen.queryByRole('dialog', { name: messageWith('dice.edit', { label }) }) ??
  screen.queryByRole('dialog', { name: messageWith('dice.editSection', { label }) });
const expressionField = (label: string) => {
  const dialog = editorDialog(label);
  if (dialog === null) throw new Error(`no dice editor is open for ${label}`);
  return within(dialog).getByRole('textbox', { name: message('dice.expressionLabel') });
};
const panels = () => document.querySelectorAll('.lg-dice-panel');
const notes = () => screen.queryAllByText(message('dice.sectionNote'));

afterEach(() => {
  cleanup();
});

describe('sheet dice: Section header', () => {
  it('Edge: no Section header repeats the note; it is drawn once, in the open Section editor', async () => {
    const { user } = await open();

    expect(notes().length).toBe(0);

    await user.click(sectionEditButton('Alpha') as HTMLElement);

    expect(notes().length).toBe(1);
  });

  it('Edge: the Section button reads Dice, opens a dialog and still names its Section', async () => {
    await open();

    const button = sectionEditButton('Alpha') as HTMLElement;

    expect(button).toHaveTextContent(message('dice.sectionButton'));
    expect(button?.getAttribute('aria-haspopup')).toBe('dialog');
    expect(button).toHaveAccessibleName('Dice for the Alpha section');
  });

  it('Edge: the Section note and its editor are not drawn while arranging', async () => {
    const { user } = await open();

    await user.click(screen.getByRole('button', { name: message('arrange.start') }));

    expect(notes().length).toBe(0);
  });

  it('Happy path: a Section expression is saved at once through PUT /dice under the section key', async () => {
    const { stub, user } = await open();

    await user.click(sectionEditButton('Alpha') as HTMLElement);
    const field = expressionField('Alpha');
    await user.clear(field);
    await user.type(field, '1d6{Enter}');

    await waitFor(() => expect(stub.puts().length).toBe(1));
    expect(stub.puts()[0]?.body).toBe('{"dice":{"section":{"alpha":"1d6"},"row":{}}}');
  });
});

describe('sheet dice: long-press on the Roll button', () => {
  it('Happy path: a long press on Roll opens the editor and does not roll', async () => {
    const { user } = await open();
    const button = rollButton('One') as HTMLElement;

    fireEvent.pointerDown(button);
    await act(() => new Promise((resolve) => setTimeout(resolve, LONG_PRESS_MS + 50)));
    fireEvent.pointerUp(button);
    fireEvent.click(button);

    expect(expressionField('One')).toBeTruthy();
    expect(panels().length).toBe(0);
    await user.keyboard('{Escape}');
  });

  it('Edge: a short tap still rolls and opens no editor', async () => {
    await open();
    const button = rollButton('One') as HTMLElement;

    fireEvent.pointerDown(button);
    fireEvent.pointerUp(button);
    fireEvent.click(button);

    expect(panels().length).toBe(1);
    expect(editorDialog('One')).toBeNull();
  });
});

describe('sheet dice: "no dice" has a way back', () => {
  it('Happy path: a Row turned off on the sheet keeps its Edit button and no Roll button', async () => {
    await open({ dice: { section: {}, row: { 'alpha.one': null } } });

    expect(rollButton('One')).toBeNull();
    expect(editButton('One')).not.toBeNull();
  });

  it('Edge: a Row silenced by a "no dice" Section keeps its Edit button so the Row can be set again', async () => {
    await open({ dice: { section: { alpha: null }, row: {} } });

    expect(rollButton('One')).toBeNull();
    expect(editButton('One')).not.toBeNull();
  });

  it('Edge: a Row with no applicable expression at all still shows no button', async () => {
    await open();

    expect(editButton('Two')).toBeNull();
  });
});

describe('sheet dice: rolling with unsaved values', () => {
  it('Happy path: a Roll reads the unsaved draft over the saved value', async () => {
    const { user } = await open();

    const two = screen.getByLabelText('Two');
    await user.clear(two);
    await user.type(two, '100');
    await user.click(rollButton('One') as HTMLElement);

    const totalText = document.querySelector('.lg-dice-panel__total-value')?.textContent;
    const total = Number(totalText);
    expect(total).toBeGreaterThanOrEqual(101);
    expect(total).toBeLessThanOrEqual(120);
  });

  it('Error: a Field the Roll reads with no number shows the explicit no-value message', async () => {
    const { user } = await open({ data: { alpha: { one: 5, two: 'abc' } } });

    await user.click(rollButton('One') as HTMLElement);

    expect(screen.getByText(message('dice.error.noValue'))).toBeTruthy();
    expect(document.querySelector('.lg-dice-panel__total-value')).toBeNull();
  });
});

describe('sheet dice: network failure', () => {
  it('Error: a PUT that never reaches the server reverts, shows the save error and marks nothing unsaved', async () => {
    const { user } = await open({ putOutcomes: [{ failure: new Error('offline') }] });

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');

    expect((await screen.findByRole('alert')).textContent).toBe(message('dice.error.saveFailed'));
    expect(rollButton('One')).not.toBeNull();
    expect(screen.queryByText(message('sheet.pendingEdits'))).toBeNull();
  });
});

describe('sheet dice: persistence across reload', () => {
  it('Happy path: a saved expression is shown again after the sheet is opened anew', async () => {
    const first = await open();
    await first.user.click(editButton('One') as HTMLElement);
    await first.user.clear(expressionField('One'));
    await first.user.type(expressionField('One'), '1d4+alpha.two{Enter}');
    await waitFor(() => expect(first.stub.puts().length).toBe(1));
    const saved = JSON.parse(first.stub.puts()[0]?.body ?? '{}') as { dice: unknown };
    cleanup();

    const second = await open({ dice: saved.dice });
    await second.user.click(editButton('One') as HTMLElement);

    expect(
      expressionField('One').getAttribute('value') ??
        (expressionField('One') as HTMLInputElement).value,
    ).toBe('1d4+alpha.two');
  });
});

describe('sheet dice: keyboard', () => {
  it('Happy path: Enter on Roll shows the panel and Escape dismisses it', async () => {
    const { user } = await open();

    rollButton('One')?.focus();
    await user.keyboard('{Enter}');
    expect(panels().length).toBe(1);

    await user.keyboard('{Escape}');
    expect(panels().length).toBe(0);
  });

  it('Edge: Escape in the editor closes it and returns focus to the Edit button', async () => {
    const { user } = await open();

    editButton('One')?.focus();
    await user.keyboard('{Enter}');
    expect(expressionField('One')).toBeTruthy();

    await user.keyboard('{Escape}');

    expect(editorDialog('One')).toBeNull();
    expect(document.activeElement).toBe(editButton('One'));
  });
});
