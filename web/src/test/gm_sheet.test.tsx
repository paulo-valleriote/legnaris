import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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

const GM = `/campaigns/${CAMPAIGN_ID}/sheets/${SHEET_ID}`;
const TEMPLATE = 'fantasy_v1';
const ETAG_A = '"3.1.gm"';
const ETAG_B = '"4.1.gm"';

function sheetRoutes(responders: Route['respond'][]): Route[] {
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
      path: GM,
      respond: (request) => {
        const respond = responders[Math.min(index, responders.length - 1)];
        index += 1;
        return respond ? respond(request) : fail('NOT_FOUND', 404);
      },
    },
  ];
}

function freshSheet(etag: string, name = 'Brenna') {
  return {
    status: 200,
    body: JSON.stringify({ data: { ...capturedSheet(), name } }),
    etag,
  };
}

const notModified = { status: 304, body: '', etag: ETAG_A };

function setVisibility(state: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
  document.dispatchEvent(new Event('visibilitychange', { bubbles: true }));
}

describe('GM read-only sheet view (LGN-218)', () => {
  beforeEach(() => {
    vi.useRealTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    setVisibility('visible');
  });

  it('Happy path: shows the linked sheet read-only with the quiet real-time caption and a refresh control', async () => {
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A)]));
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByRole('heading', { name: 'Brenna', level: 2 })).toBeInTheDocument();
    expect(screen.getByText(message('gm.caption'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('gm.refresh') })).toBeEnabled();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(script.calls('GET', GM)[0]?.request.headers).toEqual({});
  });

  it('Happy path: the first read renders a leaf value from the template', async () => {
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A)]));
    renderApp(GM, { client: clientFor(script.transport) });

    const leaf = firstLeaf();
    expect(await screen.findByText(leaf.label)).toBeInTheDocument();
  });

  it('Happy path: a manual refresh sends a conditional read with the last ETag and keeps a 304 view', async () => {
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A), () => notModified]));
    const user = userEvent.setup();
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Brenna', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));

    await waitFor(() => expect(script.calls('GET', GM)).toHaveLength(2));
    expect(script.calls('GET', GM)[1]?.request.headers).toEqual({ 'if-none-match': ETAG_A });
    expect(screen.getByRole('heading', { name: 'Brenna', level: 2 })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('Happy path: a changed sheet on refresh replaces the shown name and sends the new ETag next time', async () => {
    const script = scripted(
      sheetRoutes([
        () => freshSheet(ETAG_A),
        () => freshSheet(ETAG_B, 'Brenna the Bold'),
        () => notModified,
      ]),
    );
    const user = userEvent.setup();
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Brenna', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));
    expect(
      await screen.findByRole('heading', { name: 'Brenna the Bold', level: 2 }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));
    await waitFor(() => expect(script.calls('GET', GM)).toHaveLength(3));
    expect(script.calls('GET', GM)[2]?.request.headers).toEqual({ 'if-none-match': ETAG_B });
  });

  it('Happy path: polls every 60 seconds with If-None-Match while the tab is visible', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A), () => notModified]));
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Brenna', level: 2 });
    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });

    await waitFor(() => expect(script.calls('GET', GM)).toHaveLength(2));
    expect(script.calls('GET', GM)[1]?.request.headers).toEqual({ 'if-none-match': ETAG_A });
    expect(screen.getByRole('heading', { name: 'Brenna', level: 2 })).toBeInTheDocument();
  });

  it('Edge: a hidden tab does not poll, and becoming visible again refreshes once', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A), () => notModified]));
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Brenna', level: 2 });
    setVisibility('hidden');
    await act(async () => {
      vi.advanceTimersByTime(180_000);
    });
    expect(script.calls('GET', GM)).toHaveLength(1);

    setVisibility('visible');
    await waitFor(() => expect(script.calls('GET', GM)).toHaveLength(2));
  });

  it('Edge: the first read carries no If-None-Match because nothing is cached yet', async () => {
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A)]));
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Brenna', level: 2 });
    expect(script.calls('GET', GM)[0]?.request.headers['if-none-match']).toBeUndefined();
  });

  it('Error: a sheet that is no longer linked shows the not-found copy on the refresh', async () => {
    const script = scripted(sheetRoutes([() => freshSheet(ETAG_A), () => fail('NOT_FOUND', 404)]));
    const user = userEvent.setup();
    renderApp(GM, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Brenna', level: 2 });
    await user.click(screen.getByRole('button', { name: message('gm.refresh') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.notFound'));
  });

  it('Error: losing access to the Campaign shows its not-found copy and keeps no sheet values', async () => {
    const script = scripted(sheetRoutes([() => fail('CAMPAIGN_NOT_FOUND', 404)]));
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.campaignNotFound'),
    );
    expect(screen.queryByRole('heading', { name: 'Brenna', level: 2 })).not.toBeInTheDocument();
  });

  it('Error: a player reading another player sheet is refused with the forbidden copy', async () => {
    const script = scripted(sheetRoutes([() => fail('CAMPAIGN_FORBIDDEN', 403)]));
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.campaignForbidden'),
    );
  });

  it('Edge: a 304 on a read with no cached copy is treated as drift, never shown as a blank sheet', async () => {
    const script = scripted(sheetRoutes([() => ({ status: 304, body: '' })]));
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.parse'));
  });
});

function firstLeaf(): { label: string } {
  const templates = capturedTemplates() as Array<{ fields: Record<string, unknown> }>;
  const first = templates[0];
  const walk = (nodes: Record<string, unknown>): string | undefined => {
    for (const node of Object.values(nodes)) {
      const n = node as {
        kind?: string;
        schema?: { label: string };
        children?: Record<string, unknown>;
      };
      if (n.schema !== undefined) {
        return n.schema.label;
      }
      if (n.children !== undefined) {
        const found = walk(n.children);
        if (found !== undefined) {
          return found;
        }
      }
    }
    return undefined;
  };
  const label = first === undefined ? undefined : walk(first.fields);
  return { label: label ?? '' };
}
