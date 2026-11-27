import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import {
  CAMPAIGN_ID,
  PLAYER_ID,
  SHEET_ID,
  baseRoutes,
  campaignView,
  capturedSheet,
  capturedTemplates,
  clientFor,
  fail,
  ok,
  scripted,
} from './campaign_helpers';
import type { Route } from './campaign_helpers';
import { renderApp } from './render';

const DETAIL = `/campaigns/${CAMPAIGN_ID}`;
const NPCS = `${DETAIL}/npcs`;
const NPC = `${NPCS}/${SHEET_ID}`;
const TEMPLATE = 'fantasy_v1';
const ETAG_A = '"5.1.player"';
const ETAG_B = '"6.1.player"';

function playerView() {
  return campaignView({
    viewer_role: 'player',
    viewer_owner: false,
    viewer_user_id: PLAYER_ID,
  });
}

function npcLink(overrides: Record<string, unknown> = {}) {
  return {
    id: '0190c5a0-0000-7000-8000-00000000c009',
    sheet_id: SHEET_ID,
    name: 'Goblin',
    kind: 'npc',
    owner_id: '0190c5a0-0000-7000-8000-00000000c004',
    owner_name: 'mara',
    revealed: true,
    linked_at: '2026-11-17T10:00:00Z',
    is_owner: false,
    ...overrides,
  };
}

function freshNpc(etag: string, name = 'Goblin') {
  return {
    status: 200,
    body: JSON.stringify({
      data: { ...capturedSheet(), id: SHEET_ID, name, kind: 'npc' },
    }),
    etag,
  };
}

function setVisibility(state: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
  document.dispatchEvent(new Event('visibilitychange', { bubbles: true }));
}

function readRoutes(responders: Route['respond'][]): Route[] {
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

describe('player read-only NPC and Enemy view (LGN-205)', () => {
  afterEach(() => {
    vi.useRealTimers();
    setVisibility('visible');
  });

  it('Happy path: a player reads a revealed NPC read-only with the polling caption and refresh', async () => {
    const script = scripted(readRoutes([() => freshNpc(ETAG_A)]));
    renderApp(NPC, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('heading', { name: message('gm.npcHeading'), level: 1 }),
    ).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Goblin', level: 2 })).toBeInTheDocument();
    expect(screen.getByText(message('gm.caption'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('gm.refresh') })).toBeEnabled();
    expect(screen.queryAllByRole('textbox').every((input) => input.hasAttribute('readonly'))).toBe(
      true,
    );
    expect(script.calls('GET', NPC)).toHaveLength(1);
  });

  it('Happy path: a player refresh keeps the current sheet on a 304', async () => {
    const script = scripted(
      readRoutes([() => freshNpc(ETAG_A), () => ({ status: 304, body: '', etag: ETAG_A })]),
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

  it('Happy path: a changed sheet replaces the values and sends the new ETag next time', async () => {
    const script = scripted(
      readRoutes([
        () => freshNpc(ETAG_A),
        () => freshNpc(ETAG_B, 'Goblin the Bold'),
        () => ({ status: 304, body: '', etag: ETAG_B }),
      ]),
    );
    const user = userEvent.setup();
    renderApp(NPC, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Goblin', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));
    expect(
      await screen.findByRole('heading', { name: 'Goblin the Bold', level: 2 }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));
    await waitFor(() => expect(script.calls('GET', NPC)).toHaveLength(3));
    expect(script.calls('GET', NPC)[2]?.request.headers).toEqual({ 'if-none-match': ETAG_B });
  });

  it('Edge: polling pauses while the tab is hidden and refreshes once when visible', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const script = scripted(
      readRoutes([() => freshNpc(ETAG_A), () => ({ status: 304, body: '', etag: ETAG_A })]),
    );
    renderApp(NPC, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Goblin', level: 2 });
    setVisibility('hidden');
    await act(async () => {
      vi.advanceTimersByTime(180_000);
    });
    expect(script.calls('GET', NPC)).toHaveLength(1);

    setVisibility('visible');
    await waitFor(() => expect(script.calls('GET', NPC)).toHaveLength(2));
  });

  it('Edge: a hidden sheet shows the not-available state instead of values', async () => {
    const script = scripted(readRoutes([() => freshNpc(ETAG_A), () => fail('NOT_FOUND', 404)]));
    const user = userEvent.setup();
    renderApp(NPC, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Goblin', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('gm.npcUnavailable'));
    expect(screen.queryByRole('heading', { name: 'Goblin', level: 2 })).not.toBeInTheDocument();
  });

  it('Edge: a hidden sheet stops polling after the not-available answer', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const script = scripted(readRoutes([() => freshNpc(ETAG_A), () => fail('NOT_FOUND', 404)]));
    const user = userEvent.setup();
    renderApp(NPC, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Goblin', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));
    await screen.findByRole('alert');

    await act(async () => {
      vi.advanceTimersByTime(180_000);
    });
    expect(script.calls('GET', NPC)).toHaveLength(2);
  });

  it('Happy path: a player opens a revealed NPC, and a hide ends access and drops it from the list', async () => {
    let links: unknown[] = [npcLink()];
    let sheetAnswer: Route['respond'] = () => freshNpc(ETAG_A);
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: `/templates/${TEMPLATE}`,
        respond: () => ok(capturedTemplates()[0]),
      },
      { method: 'GET', path: DETAIL, respond: () => ok(playerView()) },
      { method: 'GET', path: NPCS, respond: () => ok(links) },
      { method: 'GET', path: '/sheets/owned', respond: () => ok([]) },
      { method: 'GET', path: NPC, respond: (request) => sheetAnswer(request) },
    ]);
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await user.click(
      await screen.findByRole('link', {
        name: messageWith('campaign.openNpc', { name: 'Goblin' }),
      }),
    );

    expect(await screen.findByRole('heading', { name: 'Goblin', level: 2 })).toBeInTheDocument();
    expect(screen.queryAllByRole('textbox').every((input) => input.hasAttribute('readonly'))).toBe(
      true,
    );

    links = [];
    sheetAnswer = () => fail('NOT_FOUND', 404);
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));
    expect(await screen.findByRole('alert')).toHaveTextContent(message('gm.npcUnavailable'));

    await user.click(screen.getByRole('link', { name: message('campaign.back') }));

    expect(
      await screen.findByRole('heading', { name: playerView().name, level: 1 }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.queryByRole('link', {
          name: messageWith('campaign.openNpc', { name: 'Goblin' }),
        }),
      ).not.toBeInTheDocument(),
    );
    expect(await screen.findByText(message('campaign.playerNpcsEmpty'))).toBeInTheDocument();
  });
});
