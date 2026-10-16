import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import type { TemplateSummary } from '../api/models/template';
import { message } from '../l10n/strings';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const CATALOG: readonly TemplateSummary[] = [
  { id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} },
  { id: 'pbta', name: 'Powered by the Apocalypse', default: false, fields: {} },
];

const ENTRY_NAME = /Dungeons & Dragons 5e/;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function detailEnvelope() {
  return successOutcome({ id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} });
}

/** Renders the catalog, opens the surface from the first entry and waits for focus. */
async function openSurface() {
  const fake = createFakeTransport(successOutcome(CATALOG), detailEnvelope());
  const user = userEvent.setup();
  renderApp('/', { client: clientWith(fake.transport) });

  const entry = await screen.findByRole('button', { name: ENTRY_NAME });
  await user.click(entry);
  await waitFor(() => expect(screen.getByLabelText(message('sheet.nameLabel'))).toHaveFocus());

  return { fake, user, entry };
}

describe('catalog keyboard and navigation (WCT-19, WCT-20, WCT-27)', () => {
  it('Happy path: activating an entry moves focus into the surface (WCT-27)', async () => {
    await openSurface();

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByLabelText(message('sheet.nameLabel'))).toHaveFocus();
  });

  it('Happy path: Esc dismisses, returns focus to the entry and leaves the catalog unchanged without a request (WCT-19, WCT-20)', async () => {
    const { fake, user, entry } = await openSurface();
    const attemptsBefore = fake.attempts;

    await user.keyboard('{Escape}');

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(entry).toHaveFocus();
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
    expect(fake.attempts).toBe(attemptsBefore);
    expect(window.location.pathname).toBe('/');
  });

  it('Happy path: the close action dismisses and returns focus to the triggering entry (WCT-19, WCT-27)', async () => {
    const { user, entry } = await openSurface();

    await user.click(screen.getByRole('button', { name: message('sheet.close') }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(entry).toHaveFocus();
  });

  it('Edge: no control traps focus inside the surface (WCT-27)', async () => {
    const { user } = await openSurface();
    const dialog = screen.getByRole('dialog');
    expect(dialog.contains(document.activeElement)).toBe(true);

    await user.tab({ shift: true });
    await user.tab({ shift: true });

    expect(dialog.contains(document.activeElement)).toBe(false);
  });
});
