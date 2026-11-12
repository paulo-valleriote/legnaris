import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { renderApp } from './render';
import { rollName } from './dice_names';
import { arrangeTemplatePayload, createArrangeStub } from './arrange_stub';
import { errorOutcome } from './fake_transport';
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

const rollButton = (label: string) => screen.queryByRole('button', { name: rollName(label) });
const editButton = (label: string) =>
  screen.queryByRole('button', { name: messageWith('dice.edit', { label }) });
const editorDialog = (label: string) =>
  screen.queryByRole('dialog', { name: messageWith('dice.edit', { label }) }) ??
  screen.queryByRole('dialog', { name: messageWith('dice.editSection', { label }) });
const expressionField = (label: string) => {
  const dialog = editorDialog(label);
  if (dialog === null) throw new Error(`no dice editor is open for ${label}`);
  return within(dialog).getByRole('textbox', { name: message('dice.expressionLabel') });
};
const panels = () => document.querySelectorAll('.lg-dice-panel');

afterEach(() => {
  cleanup();
});

describe('sheet dice: rolling a Row', () => {
  it('Happy path: tapping the Roll button of a seeded Row shows the total and the breakdown', async () => {
    const { user } = await open();

    await user.click(rollButton('One') as HTMLElement);

    await waitFor(() => expect(panels().length).toBe(1));
    const value = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(document.querySelector('.lg-dice-panel__total-label')?.textContent).toBe('One');
    expect(value).toBeGreaterThanOrEqual(4);
    expect(value).toBeLessThanOrEqual(23);
    expect(panels().length).toBe(1);
    expect(panels()[0]?.textContent).toContain('alpha.two');
  });

  it('Happy path: the Roll button shows the dice it throws and names them after the Row', async () => {
    await open();

    const button = rollButton('One') as HTMLElement;

    expect(button.textContent).toBe('d20');
    expect(button).toHaveAccessibleName('Roll One, d20');
  });

  it('Edge: a Roll that throws no dice shows the plain word and the plain name', async () => {
    await open({ dice: { section: {}, row: { 'alpha.one': 'alpha.two+3' } } });

    const button = rollButton('One') as HTMLElement;

    expect(button.textContent).toBe(message('dice.rollShort'));
    expect(button).toHaveAccessibleName(messageWith('dice.roll', { label: 'One' }));
  });

  it('Edge: a Row with no applicable expression shows no Roll or Edit button', async () => {
    await open();

    expect(rollButton('Two')).toBeNull();
    expect(editButton('Two')).toBeNull();
    expect(rollButton('Three')).toBeNull();
  });

  it('Edge: rapid taps on Roll never stack panels', async () => {
    const { user } = await open();
    const button = rollButton('One') as HTMLElement;

    await user.click(button);
    await user.click(button);
    await user.click(button);

    expect(panels().length).toBe(1);
  });

  it('Edge: Escape dismisses the panel and the sheet stays usable', async () => {
    const { user } = await open();

    await user.click(rollButton('One') as HTMLElement);
    expect(panels().length).toBe(1);
    await user.keyboard('{Escape}');

    expect(panels().length).toBe(0);
    expect(rollButton('One')).not.toBeNull();
  });

  it('Edge: Arrange mode hides the dice buttons and leaving it restores them', async () => {
    const { user } = await open();

    await user.click(screen.getByRole('button', { name: message('arrange.start') }));
    expect(rollButton('One')).toBeNull();

    await user.click(screen.getByRole('button', { name: message('arrange.cancel') }));
    expect(rollButton('One')).not.toBeNull();
  });
});

describe('sheet dice: editing a Row expression', () => {
  it('Happy path: a valid edit is saved at once through PUT /dice and never as a PATCH', async () => {
    const { stub, user } = await open();

    await user.click(editButton('One') as HTMLElement);
    const field = expressionField('One');
    await user.clear(field);
    await user.type(field, '1d4+alpha.two{Enter}');

    await waitFor(() =>
      expect(stub.puts().some((request) => request.url.endsWith('/dice'))).toBe(true),
    );
    const put = stub.puts().find((request) => request.url.endsWith('/dice'));
    expect(put?.body).toContain('"alpha.one":"1d4+alpha.two"');
    expect(stub.patches().length).toBe(0);
  });

  it('Error: an invalid expression is refused inline with no request made', async () => {
    const { stub, user } = await open();

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d{Enter}');

    expect(await screen.findByRole('alert')).toBeTruthy();
    expect(stub.puts().filter((request) => request.url.endsWith('/dice')).length).toBe(0);
  });

  it('Error: a 400 from the server is shown and the previous expression is restored', async () => {
    const { stub, user } = await open({
      putOutcomes: [errorOutcome('INVALID_REQUEST', 'bad', 400)],
    });

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');

    expect((await screen.findByRole('alert')).textContent).toBe(
      message('dice.error.serverRefused'),
    );
    expect(stub.puts().length).toBe(1);
    expect(rollButton('One')).not.toBeNull();
  });

  it('Edge: a saved dice edit never shows the unsaved-changes status', async () => {
    const { stub, user } = await open();

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');

    await waitFor(() => expect(stub.puts().length).toBe(1));
    expect(screen.queryByText(message('sheet.pendingEdits'))).toBeNull();
  });
});
