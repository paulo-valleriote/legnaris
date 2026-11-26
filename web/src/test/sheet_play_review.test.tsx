import { startEditing } from './start_editing';
import { describe, expect, it, vi } from 'vitest';
import { cleanup, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { FANTASY_SHEET_ID, fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';

async function openPreset() {
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload()),
    successOutcome(fifthCharacterTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  await startEditing();
  return { fake, user };
}

async function openGeneric() {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload()),
    successOutcome(fantasyTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

describe('the preset review surface', () => {
  it('Edge: a preset sheet offers no sheet mode group', async () => {
    await openPreset();

    expect(screen.queryByRole('group', { name: 'Sheet mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Play mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit mode' })).not.toBeInTheDocument();
  });

  it('Edge: a preset draws no separate per-row d20 action button, the generic form does', async () => {
    await openPreset();

    expect(document.querySelectorAll('.lg-dice-actions')).toHaveLength(0);
    expect(document.querySelectorAll('.lg-dice-button--roll')).toHaveLength(0);

    cleanup();
    await openGeneric();

    expect(document.querySelectorAll('.lg-dice-actions').length).toBeGreaterThan(0);
  });

  it('Edge: a preset renders no template settings, the generic form keeps them', async () => {
    await openPreset();

    expect(screen.queryByTestId('template-settings')).not.toBeInTheDocument();
    expect(document.querySelector('[data-section-key="config"]')).toBeNull();

    cleanup();
    await openGeneric();

    const settings = screen.getByTestId('template-settings');
    expect(settings.querySelector('[data-section-key="config"]')).not.toBeNull();
  });

  it('Edge: a preset draws no footer, even with pending changes', async () => {
    const { user } = await openPreset();

    const level = within(
      document.querySelector<HTMLElement>('[data-field-path="identity.level"]') as HTMLElement,
    ).getByRole('textbox');
    await user.clear(level);
    await user.type(level, '6');

    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(document.querySelector('.lg-sheet__footer')).toBeNull();
    expect(screen.queryByTestId('sheet-status')).not.toBeInTheDocument();
  });

  it('Happy path: the generic form keeps the footer status and the jump control', async () => {
    const { user } = await openGeneric();

    expect(document.querySelector('.lg-sheet__footer')).not.toBeNull();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeInTheDocument();

    const name = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(name);
    await user.type(name, 'Rune');

    expect(screen.getByTestId('sheet-status')).toHaveTextContent(message('sheet.pendingEdits'));
  });
});
