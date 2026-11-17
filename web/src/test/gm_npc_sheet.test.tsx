import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import {
  CAMPAIGN_ID,
  SHEET_ID,
  baseRoutes,
  capturedSheet,
  capturedTemplates,
  clientFor,
  fail,
  ok,
  scripted,
} from './campaign_helpers';
import type { Route } from './campaign_helpers';
import { renderApp } from './render';

const NPC = `/campaigns/${CAMPAIGN_ID}/npcs/${SHEET_ID}`;
const TEMPLATE = 'fantasy_v1';
const ETAG_A = '"2.1.gm"';

function npcRoutes(responders: Route['respond'][]): Route[] {
  let index = 0;
  return [
    ...baseRoutes(),
    {
      method: 'GET',
      path: `/templates/${TEMPLATE}`,
      respond: () => ok(capturedTemplates()[0]),
    },
    {
      method: 'GET',
      path: NPC,
      respond: (request) => {
        const respond = responders[Math.min(index, responders.length - 1)];
        index += 1;
        return respond ? respond(request) : fail('NOT_FOUND', 404);
      },
    },
  ];
}

function freshNpc(etag: string) {
  return {
    status: 200,
    body: JSON.stringify({
      data: { ...capturedSheet(), id: SHEET_ID, name: 'Goblin', kind: 'npc' },
    }),
    etag,
  };
}

describe('GM read-only NPC and Enemy view (LGN-219)', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('Happy path: opens a linked NPC read-only through the NPC route with the quiet caption', async () => {
    const script = scripted(npcRoutes([() => freshNpc(ETAG_A)]));
    renderApp(NPC, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('heading', { name: message('gm.npcHeading'), level: 1 }),
    ).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Goblin', level: 2 })).toBeInTheDocument();
    expect(screen.getByText(message('gm.caption'))).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(script.calls('GET', NPC)).toHaveLength(1);
  });

  it('Happy path: a manual refresh sends the last ETag and keeps the view on a 304', async () => {
    const script = scripted(
      npcRoutes([() => freshNpc(ETAG_A), () => ({ status: 304, body: '', etag: ETAG_A })]),
    );
    const user = userEvent.setup();
    renderApp(NPC, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Goblin', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));

    await waitFor(() => expect(script.calls('GET', NPC)).toHaveLength(2));
    expect(script.calls('GET', NPC)[1]?.request.headers).toEqual({ 'if-none-match': ETAG_A });
    expect(screen.getByRole('heading', { name: 'Goblin', level: 2 })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('Happy path: polls every 60 seconds with If-None-Match while the tab is visible', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const script = scripted(
      npcRoutes([() => freshNpc(ETAG_A), () => ({ status: 304, body: '', etag: ETAG_A })]),
    );
    renderApp(NPC, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Goblin', level: 2 });
    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });

    await waitFor(() => expect(script.calls('GET', NPC)).toHaveLength(2));
    expect(script.calls('GET', NPC)[1]?.request.headers).toEqual({ 'if-none-match': ETAG_A });
    expect(screen.getByRole('heading', { name: 'Goblin', level: 2 })).toBeInTheDocument();
  });

  it('Error: a link that is gone or hidden shows the unavailable copy', async () => {
    const script = scripted(npcRoutes([() => fail('NOT_FOUND', 404)]));
    renderApp(NPC, { client: clientFor(script.transport) });

    expect(await screen.findByRole('alert')).toHaveTextContent(message('gm.npcUnavailable'));
    expect(screen.queryByRole('heading', { name: 'Goblin', level: 2 })).not.toBeInTheDocument();
  });
});
