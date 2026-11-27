import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { message } from '../l10n/strings';
import { baseRoutes, clientFor, fail, ok, scripted } from './campaign_helpers';
import type { Route } from './campaign_helpers';
import { renderApp } from './render';
import {
  SHEET_ID,
  TEMPLATE_ID,
  sheetData,
  sheetPayload,
  templateDetailPayload,
} from './sheet_fixtures';

const SHEET = `/sheets/${SHEET_ID}`;
const DUPLICATE = `${SHEET}/duplicate`;
const COPY_ID = '01a14236-21c9-7546-945e-40c46d1fcd1f';

function copyPayload(overrides: Record<string, unknown> = {}) {
  return {
    id: COPY_ID,
    template_id: TEMPLATE_ID,
    kind: 'npc',
    name: 'Aragorn (copy)',
    data: sheetData(),
    created_at: '2026-11-17T10:00:00Z',
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

function sheetRoutes(
  kind: string,
  duplicate: Route['respond'] = () => ok(copyPayload(), 201),
): Route[] {
  return [
    ...baseRoutes(),
    { method: 'GET', path: SHEET, respond: () => ok(sheetPayload({ kind })) },
    {
      method: 'GET',
      path: `/templates/${TEMPLATE_ID}`,
      respond: () => ok(templateDetailPayload()),
    },
    { method: 'POST', path: DUPLICATE, respond: duplicate },
  ];
}

function persistentQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
}

describe('sheet actions: Duplicate an NPC or Enemy (LGN-219)', () => {
  it('Happy path: duplicates an NPC and opens the copy without a second sheet fetch', async () => {
    const script = scripted(sheetRoutes('npc'));
    const user = userEvent.setup();
    renderApp(SHEET, { client: clientFor(script.transport), queryClient: persistentQueryClient() });

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    await user.click(screen.getByRole('button', { name: message('sheet.duplicate') }));

    await waitFor(() =>
      expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn (copy)'),
    );
    expect(script.calls('POST', DUPLICATE)).toHaveLength(1);
    expect(script.calls('GET', `/sheets/${COPY_ID}`)).toHaveLength(0);
  });

  it('Edge: a character sheet has no Duplicate action', async () => {
    const script = scripted(sheetRoutes('character'));
    renderApp(SHEET, { client: clientFor(script.transport) });

    await screen.findByTestId('sheet-name');
    expect(
      screen.queryByRole('button', { name: message('sheet.duplicate') }),
    ).not.toBeInTheDocument();
  });

  it('Error: duplicating at the per-kind limit shows the limit copy and keeps the sheet open', async () => {
    const script = scripted(sheetRoutes('npc', () => fail('PLAN_LIMIT_REACHED', 403)));
    const user = userEvent.setup();
    renderApp(SHEET, { client: clientFor(script.transport) });

    await screen.findByTestId('sheet-name');
    await user.click(screen.getByRole('button', { name: message('sheet.duplicate') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('sheet.limitReached'));
    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(script.calls('POST', DUPLICATE)).toHaveLength(1);
    expect(screen.getByRole('button', { name: message('sheet.duplicate') })).toBeEnabled();
  });

  it('Error: SHEET_NAME_INVALID shows the name-required copy and keeps the sheet open', async () => {
    const script = scripted(sheetRoutes('npc', () => fail('SHEET_NAME_INVALID', 422)));
    const user = userEvent.setup();
    renderApp(SHEET, { client: clientFor(script.transport) });

    await screen.findByTestId('sheet-name');
    await user.click(screen.getByRole('button', { name: message('sheet.duplicate') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('sheet.nameRequired'));
    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(screen.getByRole('button', { name: message('sheet.duplicate') })).toBeEnabled();
  });

  it('Edge: unsaved edits ask before duplicating, and keeping them sends nothing', async () => {
    const script = scripted(sheetRoutes('npc'));
    const user = userEvent.setup();
    renderApp(SHEET, { client: clientFor(script.transport) });

    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.duplicate') }));

    const dialog = await screen.findByRole('dialog', { name: message('sheet.leaveTitle') });
    await user.click(within(dialog).getByRole('button', { name: message('sheet.cancel') }));

    expect(script.calls('POST', DUPLICATE)).toHaveLength(0);
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('4');
  });

  it('Happy path: confirming the leave guard discards the edits and duplicates', async () => {
    const script = scripted(sheetRoutes('npc'));
    const user = userEvent.setup();
    renderApp(SHEET, { client: clientFor(script.transport), queryClient: persistentQueryClient() });

    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.duplicate') }));

    const dialog = await screen.findByRole('dialog', { name: message('sheet.leaveTitle') });
    await user.click(within(dialog).getByRole('button', { name: message('sheet.leaveConfirm') }));

    await waitFor(() =>
      expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn (copy)'),
    );
    expect(script.calls('POST', DUPLICATE)).toHaveLength(1);
  });
});
