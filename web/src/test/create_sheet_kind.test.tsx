import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserEvent } from '@testing-library/user-event';
import { failureMessage } from '../api/error_mapper';
import { createApiClient } from '../api/client';
import type { HttpRequest, Transport } from '../api/transport';
import type { SheetKind } from '../api/models/plan';
import { message, messageWith } from '../l10n/strings';
import { createFakeTransport, errorOutcome, successOutcome, withPlanUsage } from './fake_transport';
import { renderApp } from './render';
import { TEMPLATE_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const ENTRY_NAME = /Fantasy/;

const CATALOG = [{ id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} }];

const KIND_LIMITS = {
  character: 1,
  npc: 2,
  enemy: 2,
} as const satisfies Record<SheetKind, number>;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function usageFor(kind: SheetKind, fullKinds: readonly SheetKind[]) {
  const limit = KIND_LIMITS[kind];
  return { used: fullKinds.includes(kind) ? limit : 0, limit };
}

function planUsage(...fullKinds: readonly SheetKind[]) {
  return {
    plan: 'free',
    sheets: {
      character: usageFor('character', fullKinds),
      npc: usageFor('npc', fullKinds),
      enemy: usageFor('enemy', fullKinds),
    },
  };
}

async function openSurface(
  user: UserEvent,
  plan = planUsage(),
  fake = createFakeTransport(successOutcome(CATALOG), successOutcome(templateDetailPayload())),
) {
  renderApp('/', { client: clientWith(withPlanUsage(fake.transport, successOutcome(plan))) });
  await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
  await screen.findByTestId('detail-name');
  return fake;
}

function kindSelect() {
  return screen.getByLabelText(message('catalog.kindLabel'));
}

function nameInput() {
  return screen.getByLabelText(message('sheet.nameLabel'));
}

function createButton() {
  return screen.getByRole('button', { name: message('sheet.create') });
}

function generateLink() {
  return screen.getByRole('link', { name: message('generate.entry') });
}

function generateButton() {
  return screen.getByRole('button', { name: message('generate.entry') });
}

function postRequests(requests: readonly HttpRequest[]) {
  return requests.filter((request) => request.method === 'POST' && request.url.endsWith('/sheets'));
}

describe('creation surface sheet kind (LGN-238)', () => {
  it('Edge: the default kind is character and its allowance is the one shown', async () => {
    const user = userEvent.setup();
    await openSurface(user, planUsage('npc'));

    expect(kindSelect()).toHaveValue('character');
    expect(kindSelect()).toHaveAttribute('aria-describedby', 'sheet-kind-usage');
    expect(
      await screen.findByText(messageWith('sheet.usage', usageFor('character', []))),
    ).toBeInTheDocument();
  });

  it('Edge: a full allowance of one kind disables Create for that kind only and switching kind re-enables it', async () => {
    const user = userEvent.setup();
    await openSurface(user, planUsage('npc'));

    await user.selectOptions(kindSelect(), 'npc');
    await user.type(nameInput(), 'Mine');

    expect(screen.getByTestId('sheet-limit-reached')).toHaveTextContent(
      message('sheet.limitReached'),
    );
    expect(kindSelect()).toHaveAttribute('aria-describedby', 'sheet-kind-usage sheet-kind-limit');
    expect(createButton()).toBeDisabled();
    expect(
      screen.getByText(messageWith('sheet.usage', usageFor('npc', ['npc']))),
    ).toBeInTheDocument();

    await user.selectOptions(kindSelect(), 'character');

    expect(screen.queryByTestId('sheet-limit-reached')).not.toBeInTheDocument();
    expect(kindSelect()).toHaveAttribute('aria-describedby', 'sheet-kind-usage');
    expect(createButton()).toBeEnabled();
  });

  it('Edge: two kinds at their limit block Create and the remaining kind allows it', async () => {
    const user = userEvent.setup();
    await openSurface(user, planUsage('npc', 'enemy'));

    await user.type(nameInput(), 'Mine');

    await user.selectOptions(kindSelect(), 'npc');
    expect(screen.getByTestId('sheet-limit-reached')).toBeInTheDocument();
    expect(createButton()).toBeDisabled();

    await user.selectOptions(kindSelect(), 'enemy');
    expect(screen.getByTestId('sheet-limit-reached')).toBeInTheDocument();
    expect(createButton()).toBeDisabled();

    await user.selectOptions(kindSelect(), 'character');
    expect(screen.queryByTestId('sheet-limit-reached')).not.toBeInTheDocument();
    expect(createButton()).toBeEnabled();
  });

  it('Happy path: tabbing visits the name, the kind, Generate and Create in that order', async () => {
    const user = userEvent.setup();
    await openSurface(user);

    await user.type(nameInput(), 'Mine');

    expect(nameInput()).toHaveFocus();
    await user.tab();
    expect(kindSelect()).toHaveFocus();
    await user.tab();
    expect(generateLink()).toHaveFocus();
    await user.tab();
    expect(createButton()).toHaveFocus();
  });

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
      await user.type(nameInput(), 'Mine');
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

    expect(generateLink()).toHaveAttribute('href', `/generate/${TEMPLATE_ID}?kind=character`);

    await user.selectOptions(kindSelect(), 'npc');

    expect(generateLink()).toHaveAttribute('href', `/generate/${TEMPLATE_ID}?kind=npc`);
  });

  it('Edge: Generate is disabled at the limit of the selected kind and usable after switching', async () => {
    const user = userEvent.setup();
    await openSurface(user, planUsage('npc'));

    await user.selectOptions(kindSelect(), 'npc');

    expect(generateButton()).toBeDisabled();
    expect(screen.queryByRole('link', { name: message('generate.entry') })).not.toBeInTheDocument();

    await user.selectOptions(kindSelect(), 'character');

    expect(generateLink()).toHaveAttribute('href', `/generate/${TEMPLATE_ID}?kind=character`);
  });

  it('Error: a refused creation shows the mapped copy and keeps the surface open', async () => {
    const user = userEvent.setup();
    await openSurface(
      user,
      planUsage(),
      createFakeTransport(
        successOutcome(CATALOG),
        successOutcome(templateDetailPayload()),
        errorOutcome('INVALID_REQUEST', 'sheet name is invalid', 400),
      ),
    );

    await user.type(nameInput(), 'Mine');
    await user.click(createButton());

    expect(await screen.findByTestId('create-error')).toHaveTextContent(
      failureMessage('invalidEdit'),
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('Edge: switching kind clears a previous submit error', async () => {
    const user = userEvent.setup();
    await openSurface(
      user,
      planUsage(),
      createFakeTransport(
        successOutcome(CATALOG),
        successOutcome(templateDetailPayload()),
        errorOutcome('INVALID_REQUEST', 'sheet name is invalid', 400),
      ),
    );

    await user.type(nameInput(), 'Mine');
    await user.click(createButton());
    expect(await screen.findByTestId('create-error')).toBeInTheDocument();

    await user.selectOptions(kindSelect(), 'npc');

    expect(screen.queryByTestId('create-error')).not.toBeInTheDocument();
  });
});
