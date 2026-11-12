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

describe('sheet dice: editing a Row expression (ADR 0006)', () => {
  it('Happy path: a valid edit becomes pending and goes out with Save as one PATCH, never a PUT', async () => {
    const { stub, user } = await open();

    await user.click(editButton('One') as HTMLElement);
    const field = expressionField('One');
    await user.clear(field);
    await user.type(field, '1d4+alpha.two{Enter}');

    expect(stub.patches()).toHaveLength(0);
    expect(screen.getByTestId('sheet-status')).toHaveTextContent(message('sheet.pendingEdits'));
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    await waitFor(() => expect(stub.patches()).toHaveLength(1));
    expect(JSON.parse(stub.patches()[0]?.body ?? '{}')).toEqual({
      dice: { section: {}, row: { 'alpha.one': '1d4+alpha.two' } },
    });
    expect(stub.puts()).toHaveLength(0);
    await waitFor(() => expect(screen.queryByTestId('sheet-status')).toBeNull());
  });

  it('Edge: a pending choice marks its Row "Modified" until it is saved', async () => {
    const { user } = await open();
    const row = () => document.querySelector('[data-row-key="alpha.one"]') as HTMLElement;
    expect(within(row()).queryByText(message('engine.modified'))).toBeNull();

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');

    expect(within(row()).getByText(message('engine.modified'))).toBeTruthy();
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));
    await waitFor(() => expect(within(row()).queryByText(message('engine.modified'))).toBeNull());
  });

  it('Edge: a Roll uses the pending expression before it is saved', async () => {
    const { user } = await open();

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');

    expect(rollButton('One')?.textContent).toContain('d4');
  });

  it('Error: an invalid expression is refused inline, nothing is pending and no request is made', async () => {
    const { stub, user } = await open();

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d{Enter}');

    expect(await screen.findByRole('alert')).toBeTruthy();
    expect(screen.queryByTestId('sheet-status')).toBeNull();
    expect(stub.patches()).toHaveLength(0);
  });

  it('Error: a 400 on Save shows the footer error and keeps the choice pending', async () => {
    const { stub, user } = await open({
      patchOutcomes: [errorOutcome('INVALID_REQUEST', 'bad', 400)],
    });

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    expect(await screen.findByTestId('sheet-save-error')).toHaveTextContent(
      message('api.error.invalidEdit'),
    );
    expect(stub.patches()).toHaveLength(1);
    expect(screen.getByTestId('sheet-status')).toHaveTextContent(message('sheet.pendingEdits'));
    expect(rollButton('One')?.textContent).toContain('d4');
  });

  it('Edge: Ctrl+S saves a dice-only change', async () => {
    const { stub, user } = await open();

    await user.click(editButton('One') as HTMLElement);
    await user.clear(expressionField('One'));
    await user.type(expressionField('One'), '1d4{Enter}');
    await user.keyboard('{Control>}s{/Control}');

    await waitFor(() => expect(stub.patches()).toHaveLength(1));
  });
});
