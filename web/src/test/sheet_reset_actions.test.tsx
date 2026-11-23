import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { HttpResponse } from '../api/transport';
import { message, messageWith } from '../l10n/strings';
import {
  CAPABILITIES_ACTION_LABEL,
  CAPABILITIES_LIST_PATH,
  CAPABILITIES_RECORD_ACTIVE,
  CAPABILITIES_SHEET_ID,
  capabilitiesData,
  capabilitiesResetActions,
  capabilitiesResetCascade,
  capabilitiesSheetPayload,
  capabilitiesTemplatePayload,
} from './capabilities_fixture';
import { createCapabilitiesStub } from './capabilities_stub';
import { fantasySheetPayload, fantasyTemplatePayload, FANTASY_SHEET_ID } from './fantasy_fixtures';
import { CAMPAIGN_ID, baseRoutes, clientFor, ok, scripted } from './campaign_helpers';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${CAPABILITIES_SHEET_ID}`;

function resetButton(): HTMLElement {
  return screen.getByRole('button', { name: CAPABILITIES_ACTION_LABEL });
}

function requestIDs(requests: readonly { readonly body?: string }[]): string[] {
  return requests.map((request) => JSON.parse(request.body ?? '{}')['request_id'] as string);
}

function levelField(): HTMLElement {
  return screen.getByRole('textbox', { name: 'Level' });
}

function recordRows(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-row-kind="record"]'));
}

async function open(options: Parameters<typeof createCapabilitiesStub>[0] = {}) {
  const stub = createCapabilitiesStub(options);
  const user = userEvent.setup();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { stub, user, view };
}

afterEach(cleanup);

describe('reset actions on the generic form', () => {
  it('Happy path: Enter on an action applies it, announces it and shows the cascade', async () => {
    const { stub, user, view } = await open({
      resets: [successOutcome(capabilitiesResetCascade())],
    });

    expect(resetButton()).toBeInTheDocument();
    resetButton().focus();
    await user.keyboard('{Enter}');

    await waitFor(() => expect(stub.resetRequests()).toHaveLength(1));
    expect(requestIDs(stub.resetRequests())[0]).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
    expect(levelField()).toHaveValue('2');
    expect(screen.getByTestId('sheet-reset-notice')).toHaveTextContent(
      messageWith('sheet.reset.applied', { label: CAPABILITIES_ACTION_LABEL }),
    );
    expect(screen.queryByText(message('sheet.pendingEdits'))).toBeNull();
    expect(
      within(recordRows()[0] as HTMLElement).getByRole('textbox', { name: 'Quantity' }),
    ).toHaveValue('3');
    await waitFor(async () => {
      const cached = await view.sheetStore.readSheet(CAPABILITIES_SHEET_ID);
      expect(cached?.data).toMatchObject({
        stats: { level: 2 },
        inventory: { items: { [CAPABILITIES_RECORD_ACTIVE]: { quantity: 3 } } },
        totals: { inventoryValue: 7 },
      });
    });
  });

  it('Edge: the bar draws one button per declared action, in Template order', async () => {
    await open({
      template: capabilitiesTemplatePayload({
        actions: [
          ...capabilitiesResetActions(),
          {
            id: 'prepare',
            label: 'Prepare',
            assignments: [{ target: 'stats.proficiency', expression: 'stats_proficiency' }],
          },
        ],
      }),
    });

    const group = screen.getByRole('group', { name: message('sheet.reset.group') });
    expect(
      within(group)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual([CAPABILITIES_ACTION_LABEL, 'Prepare']);
  });

  it('Edge: the bar is hidden when the Template declares no action', async () => {
    const fake = createFakeTransport(
      successOutcome(fantasySheetPayload()),
      successOutcome(fantasyTemplatePayload()),
    );
    const user = userEvent.setup();
    renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });

    await screen.findByTestId('sheet-name');
    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(screen.queryByTestId('sheet-reset-actions')).toBeNull();
    expect(screen.queryByRole('group', { name: message('sheet.reset.group') })).toBeNull();
  });

  it('Edge: a read-only campaign sheet draws no reset action', async () => {
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/templates/capabilities_test_v1',
        respond: () => ok(capabilitiesTemplatePayload()),
      },
      {
        method: 'GET',
        path: `/campaigns/${CAMPAIGN_ID}/sheets/${CAPABILITIES_SHEET_ID}`,
        respond: () => ok(capabilitiesSheetPayload({ name: 'Quartermaster' })),
      },
    ]);
    renderApp(`/campaigns/${CAMPAIGN_ID}/sheets/${CAPABILITIES_SHEET_ID}`, {
      client: clientFor(script.transport),
    });

    await screen.findByRole('heading', { name: 'Quartermaster', level: 2 });
    expect(screen.queryByRole('button', { name: CAPABILITIES_ACTION_LABEL })).toBeNull();
    expect(screen.queryByTestId('sheet-reset-actions')).toBeNull();
  });

  it('Edge: a pending draft disables the action with a visible reason and sends nothing', async () => {
    const { stub, user } = await open();

    await user.clear(levelField());
    await user.type(levelField(), '7');

    expect(resetButton()).toHaveAttribute('aria-disabled', 'true');
    expect(resetButton()).toHaveAccessibleDescription(message('sheet.reset.pending'));
    await user.click(resetButton());

    expect(stub.resetRequests()).toHaveLength(0);
  });

  it('Edge: a double tap sends exactly one request', async () => {
    let release: (response: HttpResponse) => void = () => undefined;
    const { stub, user } = await open({
      resets: [
        () =>
          new Promise<HttpResponse>((resolve) => {
            release = resolve;
          }),
      ],
    });

    await user.click(resetButton());
    expect(resetButton()).toHaveAttribute('aria-disabled', 'true');
    expect(resetButton()).toHaveAccessibleDescription(message('sheet.reset.inFlight'));
    await user.click(resetButton());

    release(successOutcome(capabilitiesResetCascade()));
    await waitFor(() =>
      expect(screen.getByTestId('sheet-reset-notice')).toHaveTextContent(
        messageWith('sheet.reset.applied', { label: CAPABILITIES_ACTION_LABEL }),
      ),
    );
    expect(stub.resetRequests()).toHaveLength(1);
  });

  it('Edge: a network failure offers a retry that reuses the request id', async () => {
    const { stub, user } = await open({
      resets: [{ failure: new Error('network') }, successOutcome(capabilitiesResetCascade())],
    });

    await user.click(resetButton());
    expect(await screen.findByTestId('sheet-reset-error')).toHaveTextContent(
      message('api.error.offline'),
    );
    await user.click(screen.getByRole('button', { name: message('common.retry') }));

    await waitFor(() => expect(stub.resetRequests()).toHaveLength(2));
    const ids = requestIDs(stub.resetRequests());
    expect(ids[1]).toBe(ids[0]);
  });

  it('Edge: a 409 offers a refresh and retry that reuses the request id', async () => {
    const refreshed = capabilitiesSheetPayload({
      data: { ...capabilitiesData(), flags: { inspired: 0 } },
    });
    const { stub, user } = await open({
      resets: [errorOutcome('CONFLICT', 'stale', 409), successOutcome(capabilitiesResetCascade())],
      refreshedSheet: refreshed,
    });

    await user.click(resetButton());
    expect(await screen.findByTestId('sheet-reset-error')).toHaveTextContent(
      message('sheet.reset.conflict'),
    );

    const before = stub.requests.length;
    await user.click(
      screen.getByRole('button', { name: message('sheet.reset.retryAfterRefresh') }),
    );

    await waitFor(() => expect(stub.resetRequests()).toHaveLength(2));
    const refreshedAt = stub.requests.findIndex(
      (request, index) =>
        index >= before && request.method === 'GET' && new URL(request.url).pathname === SHEET_URL,
    );
    const retriedAt = stub.requests.findIndex(
      (request, index) => index >= before && request.method === 'POST',
    );
    expect(refreshedAt).toBeGreaterThanOrEqual(before);
    expect(retriedAt).toBeGreaterThan(refreshedAt);
    expect(stub.sheetFetches()).toBe(2);
    expect(requestIDs(stub.resetRequests())[1]).toBe(requestIDs(stub.resetRequests())[0]);
    await waitFor(() =>
      expect(screen.getByRole('checkbox', { name: 'Inspired' })).not.toBeChecked(),
    );
    expect(levelField()).toHaveValue('2');
  });

  it('Edge: only the active Record of the fixture list is restocked', async () => {
    const { stub, user } = await open({
      resets: [successOutcome(capabilitiesResetCascade())],
    });

    await user.click(resetButton());
    await waitFor(() => expect(stub.resetRequests()).toHaveLength(1));

    expect(recordRows()[0]?.dataset.rowKey).toBe(
      `${CAPABILITIES_LIST_PATH}.${CAPABILITIES_RECORD_ACTIVE}`,
    );
    expect(
      within(recordRows()[1] as HTMLElement).getByRole('textbox', { name: 'Quantity' }),
    ).toHaveValue('1');
  });
});
