import { startEditing } from './start_editing';
import { describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import { FANTASY_SHEET_ID, fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';

async function openFifth(options: { sheetStore?: ReturnType<typeof createMemorySheetStore> } = {}) {
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload()),
    successOutcome(fifthCharacterTemplatePayload()),
    successOutcome({}),
  );
  const view = renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    ...(options.sheetStore === undefined ? {} : { sheetStore: options.sheetStore }),
  });
  await screen.findByTestId('sheet-name');
  await startEditing();
  return { fake, view, user: userEvent.setup() };
}

async function openGeneric() {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload()),
    successOutcome(
      fantasyTemplatePayload({
        actions: [
          { id: 'rest', label: 'Rest', assignments: [] },
          { id: 'prepare', label: 'Prepare', assignments: [] },
        ],
      }),
    ),
    successOutcome({}),
  );
  const view = renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, view, user: userEvent.setup() };
}

function levelInput(): HTMLElement {
  return within(
    document.querySelector<HTMLElement>('[data-field-path="identity.level"]') as HTMLElement,
  ).getByRole('textbox');
}

describe('sheet screen without a mode toggle', () => {
  it('Edge: a preset sheet draws no mode toggle and no Arrange action', async () => {
    await openFifth();

    expect(screen.queryByRole('group', { name: 'Sheet mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Play mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit mode' })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('arrange.start') }),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId('arrange-hint')).not.toBeInTheDocument();
  });

  it('Edge: a generic sheet draws no mode toggle but keeps Arrange', async () => {
    await openGeneric();

    expect(screen.queryByRole('group', { name: 'Sheet mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Play mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit mode' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('arrange.start') })).toBeInTheDocument();
  });

  it('Happy path: a generic sheet keeps the footer, Arrange and its Reset bar', async () => {
    await openGeneric();

    expect(document.querySelector('.lg-sheet__footer')).not.toBeNull();
    expect(screen.getByRole('button', { name: message('arrange.start') })).toBeInTheDocument();
    const reset = screen.getByRole('group', { name: message('sheet.reset.group') });
    expect(
      within(reset)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Rest', 'Prepare']);
  });

  it('Happy path: a preset sheet shows Help and the pending card, never the footer', async () => {
    const { user } = await openFifth();

    expect(screen.getByRole('button', { name: message('sheet.help') })).toBeInTheDocument();
    expect(document.querySelector('.lg-sheet__footer')).toBeNull();

    const level = levelInput();
    await user.clear(level);
    await user.type(level, '6');

    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(document.querySelector('.lg-sheet__footer')).toBeNull();
  });

  it('Edge: Escape leaves the pending edits in place', async () => {
    const { fake, user } = await openFifth();
    const level = levelInput();
    await user.clear(level);
    await user.type(level, '6');

    await user.keyboard('{Escape}');

    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(levelInput()).toHaveValue('6');
    expect(fake.requests.some((request) => request.method === 'PATCH')).toBe(false);
  });

  it('Edge: a stored arrangement draft does not turn a preset sheet into Arrange mode', async () => {
    const store = createMemorySheetStore();
    await store.writeLayoutDraft(FIFTH_CHARACTER_SHEET_ID, {
      sectionOrder: [],
      pinned: [],
    });

    const { fake } = await openFifth({ sheetStore: store });

    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.queryByTestId('arrange-hint')).not.toBeInTheDocument();
    expect(screen.queryByTestId('arrange-live')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('arrange.done') })).not.toBeInTheDocument();
    expect(fake.requests.some((request) => request.method === 'PUT')).toBe(false);

    await waitFor(async () => {
      expect(await store.readLayoutDraft(FIFTH_CHARACTER_SHEET_ID)).not.toBeNull();
    });
  });
});
