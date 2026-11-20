import { describe, expect, it, vi } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  FANTASY_SHEET_ID,
  fantasyFieldTree,
  fantasySheetData,
  fantasySheetPayload,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import type { FakeOutcome } from './fake_transport';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';

interface OpenFantasyOptions {
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
  readonly save?: FakeOutcome;
  readonly preview?: FakeOutcome;
}

async function openFantasy(options: OpenFantasyOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload(options.data === undefined ? {} : { data: options.data })),
    successOutcome(fantasyTemplatePayload(options.template ?? {})),
    options.preview ?? options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

async function openPlain() {
  const fake = createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(templateDetailPayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

const hero = () => screen.getByTestId('sheet-play-hero');

const pendingCard = () => screen.queryByTestId('sheet-pending-card');

function levelStep(name: 'increase' | 'decrease') {
  return screen.getByRole('button', { name: messageWith(`engine.${name}`, { label: 'Level' }) });
}

describe('Play and Edit modes', () => {
  it('Edge: a template without a preset keeps the generic form and offers no mode toggle', async () => {
    await openPlain();

    expect(screen.getByRole('textbox', { name: 'Level' })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-hero')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.mode.play') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.mode.edit') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: Play is the default for the preset and draws no generic Section or field', async () => {
    await openFantasy();

    expect(hero()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(document.querySelector('[data-section-key]')).toBeNull();
    expect(screen.queryByRole('textbox', { name: 'Character name' })).not.toBeInTheDocument();
    expect(screen.queryByTestId('section-strip')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('arrange.start') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.pinnedOnly') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: switching modes sends no request and leaves pending edits alone', async () => {
    const { fake, user } = await openFantasy();
    expect(fake.attempts).toBe(2);

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));
    expect(screen.getByRole('textbox', { name: 'Character name' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('sheet.mode.play') }));

    expect(fake.attempts).toBe(2);
    expect(hero()).toBeInTheDocument();
  });

  it('Happy path: Edit shows the generic form with Arrange and the pinned filter, and Play returns the hero', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(screen.getByRole('textbox', { name: 'Character name' })).toHaveValue('Aria');
    expect(screen.getByRole('button', { name: message('arrange.start') })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.pinnedOnly') })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-hero')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.play') }));
    expect(hero()).toBeInTheDocument();
  });
});

describe('the hero header', () => {
  it('Edge: an empty character name falls back to the sheet name, never to the empty-field copy', async () => {
    await openFantasy({ data: fantasySheetData({ characterName: '' }) });

    const title = screen.getByRole('button', { name: 'Character name' });
    expect(title).toHaveTextContent('Aragorn');
    expect(title).not.toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Character name' }),
    );
  });

  it('Edge: empty ancestry, origin and player fields read Add followed by the field label', async () => {
    await openFantasy({
      data: fantasySheetData({ ancestry: '', origin: '', playerName: '' }),
    });

    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Ancestry' }),
    );
    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Origin' }),
    );
    expect(screen.getByRole('button', { name: 'Player name' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Player name' }),
    );
  });

  it('Edge: the level stepper stops at both ends of its range', async () => {
    await openFantasy({ data: fantasySheetData({ level: 20 }) });
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 20 }),
    );
    expect(
      screen.getByRole('button', { name: messageWith('engine.increase', { label: 'Level' }) }),
    ).toBeDisabled();
    expect(
      screen.getByRole('button', { name: messageWith('engine.decrease', { label: 'Level' }) }),
    ).toBeEnabled();

    cleanup();
    await openFantasy({ data: fantasySheetData({ level: 1 }) });
    expect(
      screen.getByRole('button', { name: messageWith('engine.decrease', { label: 'Level' }) }),
    ).toBeDisabled();
  });

  it('Error: a title field the template marks non-editable keeps its stored value and writes nothing', async () => {
    const fields = fantasyFieldTree();
    const identity = fields.identity as {
      children: Record<string, { schema: Record<string, unknown> }>;
    };
    identity.children.characterName = {
      schema: { ...identity.children.characterName.schema, is_editable: false },
    };
    const { fake, user } = await openFantasy({ template: { fields } });

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(input);
    await user.type(input, 'Brenna{Enter}');

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: the hero shows the character title, ancestry, origin, level, player and proficiency', async () => {
    await openFantasy();

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent('Elf');
    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent('Wanderer');
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 3 }),
    );
    expect(hero()).toHaveTextContent('played by Sam');
    expect(hero()).toHaveTextContent(messageWith('sheet.play.proficiency', { modifier: '+2' }));
  });
});

describe('the inline edit control', () => {
  it('Edge: Escape cancels the edit and leaves no pending change', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(input);
    await user.type(input, 'Brenna{Escape}');

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: blur commits the typed value', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Ancestry' }));
    const input = screen.getByRole('textbox', { name: 'Ancestry' });
    await user.clear(input);
    await user.type(input, 'Half-elf');
    await user.tab();

    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent('Half-elf');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
  });

  it('Edge: committing an empty value writes the empty value and shows the Add copy', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Origin' }));
    const input = screen.getByRole('textbox', { name: 'Origin' });
    await user.clear(input);
    await user.keyboard('{Enter}');

    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Origin' }),
    );
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { origin: '' } } }));
  });

  it('Happy path: Enter commits the title, which writes identity.characterName only', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    expect(input).toHaveValue('Aria');
    await user.clear(input);
    await user.type(input, 'Brenna{Enter}');

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Brenna');
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { identity: { characterName: 'Brenna' } } }),
    );
  });

  it('Happy path: the level stepper writes the next level', async () => {
    const { fake, user } = await openFantasy();

    await user.click(
      screen.getByRole('button', { name: messageWith('engine.increase', { label: 'Level' }) }),
    );

    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 4 }),
    );
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 4 } } }));
  });
});

describe('the pending changes card', () => {
  it('Edge: a sheet with no edit draws no card and no table-sync slot', async () => {
    await openFantasy();

    expect(pendingCard()).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-pending-sync')).not.toBeInTheDocument();
  });

  it('Edge: the card is a polite status region', async () => {
    const { user } = await openFantasy();

    await user.click(levelStep('increase'));

    const card = screen.getByTestId('sheet-pending-card');
    expect(card).toHaveAttribute('role', 'status');
    expect(card).toHaveAttribute('aria-live', 'polite');
  });

  it('Edge: editing a value back to its stored value removes the card', async () => {
    const { fake, user } = await openFantasy();

    await user.click(levelStep('increase'));
    expect(pendingCard()).toBeInTheDocument();

    await user.click(levelStep('decrease'));

    expect(pendingCard()).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: switching to Edit mode hides the card and keeps the edits pending', async () => {
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));
    expect(pendingCard()).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.play') }));
    expect(pendingCard()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 3 → 4');
  });

  it('Edge: clearing a text field shows the empty value in the summary', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Origin' }));
    const input = screen.getByRole('textbox', { name: 'Origin' });
    await user.clear(input);
    await user.keyboard('{Enter}');

    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      `Origin Wanderer → ${message('sheet.pending.emptyValue')}`,
    );
  });

  it('Error: a failed save keeps the card and shows the mapped error', async () => {
    const { user } = await openFantasy({
      save: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });
    await user.click(levelStep('increase'));

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.save'),
      }),
    );

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(pendingCard()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 3 → 4');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
  });

  it('Error: a failed preview keeps the card and shows the mapped error', async () => {
    const { fake, user } = await openFantasy({
      preview: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });
    await user.click(levelStep('increase'));

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.preview'),
      }),
    );

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-preview-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(pendingCard()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 3 → 4');
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: two edits list each field with its old and new value and the table-sync placeholder', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(input);
    await user.type(input, 'Brenna{Enter}');
    await user.click(levelStep('increase'));

    const card = screen.getByTestId('sheet-pending-card');
    const changes = within(card).getByTestId('sheet-pending-changes');
    expect(within(changes).getAllByRole('listitem')).toHaveLength(2);
    expect(changes).toHaveTextContent('Character name Aria → Brenna');
    expect(changes).toHaveTextContent('Level 3 → 4');
    expect(
      within(changes).getByRole('listitem', {
        name: messageWith('sheet.pending.change', {
          label: 'Level',
          before: '3',
          after: '4',
        }),
      }),
    ).toBeInTheDocument();
    expect(card).toHaveTextContent(message('sheet.pending.notSavedYet'));
    expect(within(card).getByTestId('sheet-pending-sync')).toHaveTextContent(
      message('sheet.pending.tableSync'),
    );
  });

  it('Happy path: Discard restores the stored value and hides the card', async () => {
    const { fake, user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.discard'),
      }),
    );

    expect(pendingCard()).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 3 }),
    );
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: Save sends the delta once and hides the card', async () => {
    const { fake, user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.save'),
      }),
    );

    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 4 } } }));
    await waitFor(() => expect(pendingCard()).not.toBeInTheDocument());
    expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(1);
  });

  it('Happy path: Preview sends the delta and keeps the card unsaved', async () => {
    const { fake, user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.preview'),
      }),
    );

    await waitFor(() =>
      expect(fake.requests.some((request) => request.url.endsWith('/preview'))).toBe(true),
    );
    expect(pendingCard()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-card')).toHaveTextContent(
      message('sheet.pending.notSavedYet'),
    );
    expect(savedData(fake)).toBeUndefined();
  });
});
