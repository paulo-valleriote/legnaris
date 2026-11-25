import { editName } from './dice_names';
import { describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { FANTASY_SHEET_ID, fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';

async function openFantasy() {
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

async function switchToEdit(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: editName(message('sheet.mode.edit')) }));
}

describe('the Play preset review surface', () => {
  it('Happy path: the Play and Edit buttons sit in a labelled sheet mode group', async () => {
    await openFantasy();

    const group = screen.getByRole('group', { name: message('sheet.mode.group') });
    expect(
      within(group).getByRole('button', { name: editName(message('sheet.mode.play')) }),
    ).toBeInTheDocument();
    expect(
      within(group).getByRole('button', { name: editName(message('sheet.mode.edit')) }),
    ).toBeInTheDocument();
  });

  it('Edge: each quick hit point chip spells out its action as a tooltip', async () => {
    await openFantasy();

    const chip = screen.getByRole('button', {
      name: messageWith('sheet.play.damageAmount', { count: 5 }),
    });
    expect(chip).toHaveAttribute('title', messageWith('sheet.play.damageAmount', { count: 5 }));
    expect(screen.getByLabelText(message('sheet.play.amount'))).toHaveAttribute(
      'placeholder',
      message('sheet.play.amountPlaceholder'),
    );
  });

  it('Edge: Play mode draws no separate per-row d20 action button', async () => {
    const { user } = await openFantasy();

    expect(document.querySelectorAll('.lg-dice-actions')).toHaveLength(0);
    expect(document.querySelectorAll('.lg-dice-button--roll')).toHaveLength(0);

    await switchToEdit(user);

    expect(document.querySelectorAll('.lg-dice-actions').length).toBeGreaterThan(0);
  });

  it('Edge: Play mode renders no Config section, while Edit mode keeps it in the template settings', async () => {
    const { user } = await openFantasy();

    expect(document.querySelector('[data-section-key="config"]')).toBeNull();
    expect(document.querySelector('[data-row-key="config.saveDexterityProf"]')).toBeNull();
    expect(screen.queryByTestId('template-settings')).not.toBeInTheDocument();

    await switchToEdit(user);

    const settings = screen.getByTestId('template-settings');
    expect(settings.querySelector('[data-section-key="config"]')).not.toBeNull();
    expect(settings.querySelector('[data-row-key="config.saveDexterityProf"]')).not.toBeNull();
  });

  it('Edge: the passive awareness pill renders under the ability grid in Play mode', async () => {
    await openFantasy();

    const section = screen.getByRole('region', { name: 'Stats' });
    const grid = within(section).getByRole('list');
    const passive = within(section).getByTestId('sheet-play-passive');

    expect(grid.nextElementSibling).toBe(passive);
    expect(passive).toHaveTextContent('Passive awareness');
  });

  it('Edge: Play draws no footer, and Edit brings the recalculated status and jump back', async () => {
    const fake = createFakeTransport(
      successOutcome(fantasySheetPayload()),
      successOutcome(fantasyTemplatePayload()),
      successOutcome({ derived: { armorClass: 13 } }),
    );
    const user = userEvent.setup();
    renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByTestId('sheet-name');

    await user.click(screen.getByRole('button', { name: editName('Current hit points') }));
    const current = screen.getByRole('textbox', { name: 'Current hit points' });
    await user.clear(current);
    await user.type(current, '6{Enter}');
    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: editName(message('sheet.pending.preview')),
      }),
    );

    expect(document.querySelector('.lg-sheet__footer')).toBeNull();
    expect(screen.queryByTestId('sheet-status')).not.toBeInTheDocument();

    await switchToEdit(user);

    await waitFor(() =>
      expect(screen.getByTestId('sheet-status')).toHaveTextContent(
        messageWith('sheet.statusUnsavedCount', { count: 1 }),
      ),
    );
    expect(
      screen.getByRole('button', { name: editName(message('sheet.jumpToFirstName')) }),
    ).toBeInTheDocument();
  });
});
