import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { createFakeTransport, successOutcome } from './fake_transport';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { renderApp } from './render';
import { startEditing } from './start_editing';

async function open() {
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload()),
    successOutcome(fifthCharacterTemplatePayload()),
    successOutcome({}),
  );
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
  });
  await screen.findByTestId('sheet-play');
  return { fake, user: userEvent.setup() };
}

function element(selector: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(selector);
  if (found === null) {
    throw new Error(`missing ${selector}`);
  }
  return found;
}

const level = () =>
  within(element('[data-field-path="identity.level"]')).getByRole('textbox', { hidden: true });

const edited = (path: string) => element(`[data-edit-path="${path}"]`);

const toggle = (key: 'sheet.edit' | 'sheet.editDone') =>
  screen.getByRole('button', { name: message(key) });

describe('the sheet edit button', () => {
  it('Edge: a preset sheet opens with every build value locked', async () => {
    await open();

    expect(level()).toBeDisabled();
    expect(edited('identity.name').tagName).toBe('SPAN');
    expect(edited('abilities.strength.score').tagName).toBe('SPAN');
    for (const control of document.querySelectorAll('.lg-compose-flag, .lg-compose-rank')) {
      expect(control).toBeDisabled();
    }
    for (const control of document.querySelectorAll(
      '.lg-compose-record__remove, .lg-compose-record__move',
    )) {
      expect(control).toBeDisabled();
    }
  });

  it('Edge: table-play controls stay live while locked', async () => {
    await open();

    expect(edited('combat.hp.current').tagName).toBe('BUTTON');
    expect(edited('combat.hp.temp').tagName).toBe('BUTTON');
    const pips = document.querySelectorAll('.lg-compose-pip, .lg-compose-condition');
    expect(pips.length).toBeGreaterThan(0);
    for (const control of pips) {
      expect(control).toBeEnabled();
    }
    for (const row of document.querySelectorAll('[data-record-key^="resources."]')) {
      const live = row.querySelectorAll('input:not(:disabled)');
      expect(live).toHaveLength(1);
    }
  });

  it('Edge: leaving edit mode keeps the pending change and locks the field again', async () => {
    const { user } = await open();
    await startEditing();

    await user.clear(level());
    await user.type(level(), '7');
    await user.click(toggle('sheet.editDone'));

    expect(level()).toBeDisabled();
    expect(level()).toHaveValue('7');
    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
  });

  it('Edge: pressing the button sends nothing to the server', async () => {
    const { fake, user } = await open();
    const before = fake.requests.length;

    await startEditing();
    await user.click(toggle('sheet.editDone'));

    expect(fake.requests).toHaveLength(before);
    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
  });

  it('Happy path: Edit sheet unlocks the build values', async () => {
    await open();

    await startEditing();

    expect(level()).toBeEnabled();
    expect(edited('identity.name').tagName).toBe('BUTTON');
    expect(edited('abilities.strength.score').tagName).toBe('BUTTON');
    expect(toggle('sheet.editDone')).toBeInTheDocument();
  });
});
