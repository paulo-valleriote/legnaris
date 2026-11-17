import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserEvent } from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { HttpRequest, Transport } from '../api/transport';
import type { SheetKind } from '../api/models/plan';
import { message, messageWith } from '../l10n/strings';
import { createFakeTransport, successOutcome, withPlanUsage } from './fake_transport';
import { renderApp } from './render';
import { TEMPLATE_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const ENTRY_NAME = /Fantasy/;

const CATALOG = [{ id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} }];

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function usageFor(kind: SheetKind) {
  return kind === 'character' ? { used: 1, limit: 1 } : { used: 2, limit: 2 };
}

function planUsage(full?: SheetKind) {
  return {
    plan: 'free',
    sheets: {
      character: { used: full === 'character' ? 1 : 0, limit: 1 },
      npc: { used: full === 'npc' ? 2 : 0, limit: 2 },
      enemy: { used: full === 'enemy' ? 2 : 0, limit: 2 },
    },
  };
}

async function openSurface(user: UserEvent, plan = planUsage()) {
  const fake = createFakeTransport(
    successOutcome(CATALOG),
    successOutcome(templateDetailPayload()),
  );
  renderApp('/', { client: clientWith(withPlanUsage(fake.transport, successOutcome(plan))) });
  await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
  await screen.findByTestId('detail-name');
  return fake;
}

function kindSelect() {
  return screen.getByLabelText(message('catalog.kindLabel'));
}

function createButton() {
  return screen.getByRole('button', { name: message('sheet.create') });
}

function postRequests(requests: readonly HttpRequest[]) {
  return requests.filter((request) => request.method === 'POST' && request.url.endsWith('/sheets'));
}

describe('creation surface sheet kind (LGN-238)', () => {
  it('Edge: the default kind is character and its allowance is the one shown', async () => {
    const user = userEvent.setup();
    await openSurface(user, planUsage('npc'));

    expect(kindSelect()).toHaveValue('character');
    expect(
      await screen.findByText(messageWith('sheet.usage', { used: 0, limit: 1 })),
    ).toBeInTheDocument();
  });

  it.each<SheetKind>(['character', 'npc', 'enemy'])(
    'Edge: a full %s allowance disables Create for that kind only and switching kind re-enables it',
    async (full) => {
      const user = userEvent.setup();
      await openSurface(user, planUsage(full));

      await user.selectOptions(kindSelect(), full);
      await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Mine');

      expect(screen.getByTestId('sheet-limit-reached')).toHaveTextContent(
        message('sheet.limitReached'),
      );
      expect(createButton()).toBeDisabled();
      expect(screen.getByText(messageWith('sheet.usage', usageFor(full)))).toBeInTheDocument();

      const room = full === 'character' ? 'npc' : 'character';
      await user.selectOptions(kindSelect(), room);

      expect(screen.queryByTestId('sheet-limit-reached')).not.toBeInTheDocument();
      expect(createButton()).toBeEnabled();
    },
  );

  it.each<SheetKind>(['npc', 'enemy'])(
    'Happy path: creating an %s sheet sends the selected kind',
    async (kind) => {
      const fake = createFakeTransport(
        successOutcome(CATALOG),
        successOutcome(templateDetailPayload()),
        successOutcome(sheetPayload({ kind }), 201),
        successOutcome(templateDetailPayload()),
      );
      const user = userEvent.setup();
      renderApp('/', {
        client: clientWith(withPlanUsage(fake.transport, successOutcome(planUsage()))),
      });

      await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
      await screen.findByTestId('detail-name');
      await user.selectOptions(kindSelect(), kind);
      await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Mine');
      await user.click(createButton());

      await waitFor(() => {
        const posts = postRequests(fake.requests);
        expect(posts).toHaveLength(1);
        expect(JSON.parse(posts[0]?.body ?? '')).toEqual({
          template_id: TEMPLATE_ID,
          kind,
          sheet_name: 'Mine',
        });
      });
    },
  );

  it('Happy path: the Generate link carries the selected kind', async () => {
    const user = userEvent.setup();
    await openSurface(user);

    expect(screen.getByRole('link', { name: message('generate.entry') })).toHaveAttribute(
      'href',
      `/generate/${TEMPLATE_ID}?kind=character`,
    );

    await user.selectOptions(kindSelect(), 'npc');

    expect(screen.getByRole('link', { name: message('generate.entry') })).toHaveAttribute(
      'href',
      `/generate/${TEMPLATE_ID}?kind=npc`,
    );
  });
});
