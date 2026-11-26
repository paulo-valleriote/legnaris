import { startEditing } from './start_editing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { DiceFormulas } from '../sheet/preset/compose/dice_formulas';
import { PresetPlayView } from '../sheet/preset/play_view';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheet,
  fifthCharacterSheetPayload,
  fifthCharacterTemplate,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderPreset } from './preset_fixtures';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';

afterEach(cleanup);

function diceDetails(): HTMLDetailsElement {
  const details = document.querySelector<HTMLDetailsElement>(
    '[data-testid="compose-dice-formulas"]',
  );
  if (details === null) {
    throw new Error('no dice formulas details');
  }
  return details;
}

function editButtons(): HTMLButtonElement[] {
  return [
    ...document.querySelectorAll<HTMLButtonElement>('.lg-dice-actions .lg-dice-button--icon'),
  ];
}

async function openDiceFormulas(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(within(diceDetails()).getByText(message('preset.diceFormulas.title')));
  await waitFor(() => expect(diceDetails().open).toBe(true));
}

function editorExpression(): HTMLInputElement {
  const dialog = screen.getByRole('dialog');
  return within(dialog).getByRole('textbox', {
    name: message('dice.expressionLabel'),
  }) as HTMLInputElement;
}

async function editFirstFormula(
  user: ReturnType<typeof userEvent.setup>,
  expression: string,
): Promise<void> {
  const button = editButtons()[0];
  expect(button).toBeDefined();
  await user.click(button);
  const field = editorExpression();
  await user.clear(field);
  await user.type(field, expression);
  await user.click(screen.getByRole('button', { name: message('dice.save') }));
}

describe('the dice formulas piece', () => {
  it('Edge: it starts closed and draws no row controls', () => {
    renderPreset(<DiceFormulas />, {
      template: fifthCharacterTemplate(),
      sheet: fifthCharacterSheet(),
    });

    expect(diceDetails().open).toBe(false);
    expect(document.querySelectorAll('.lg-dice-actions')).toHaveLength(0);
  });

  it('Happy path: opening it shows the roll and edit controls of the rows with formulas', async () => {
    const user = userEvent.setup();
    renderPreset(<DiceFormulas />, {
      template: fifthCharacterTemplate(),
      sheet: fifthCharacterSheet(),
    });

    await openDiceFormulas(user);

    expect(editButtons().length).toBeGreaterThan(0);
    expect(document.querySelectorAll('.lg-dice-button--roll').length).toBeGreaterThan(0);
  });

  it('Happy path: editing a formula makes the pending card show the dice line', async () => {
    const user = userEvent.setup();
    const preset = presetFromConfig(fifthCharacterConfig);
    renderPreset(<PresetPlayView preset={preset} />, {
      template: fifthCharacterTemplate(),
      sheet: fifthCharacterSheet(),
    });

    await openDiceFormulas(user);
    await editFirstFormula(user, '1d6');

    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      message('sheet.pending.dice'),
    );
  });

  it('Happy path: Save sends the dice block to the server', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(fifthCharacterSheetPayload()),
      successOutcome(fifthCharacterTemplatePayload()),
      successOutcome({}),
    );
    renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByTestId('sheet-name');
    await startEditing();

    await openDiceFormulas(user);
    await editFirstFormula(user, '1d6');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));

    await waitFor(() =>
      expect(fake.requests.some((request) => request.method === 'PATCH')).toBe(true),
    );
    const patch = fake.requests.find((request) => request.method === 'PATCH');
    const body = JSON.parse(patch?.body ?? '{}') as {
      dice?: { row?: Record<string, string> };
    };
    expect(Object.values(body.dice?.row ?? {})).toContain('1d6');
  });
});
