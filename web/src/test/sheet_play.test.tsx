import { startEditing } from './start_editing';
import { editName } from './dice_names';
import { describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  PENDING_CARD_SPACE_CLASS,
  PENDING_CARD_SPACE_PROPERTY,
} from '../sheet/preset/pending_card';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import type { FakeOutcome } from './fake_transport';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';

interface OpenFifthOptions {
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
  readonly save?: FakeOutcome;
  readonly preview?: FakeOutcome;
}

async function openFifth(options: OpenFifthOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(
      fifthCharacterSheetPayload(options.data === undefined ? {} : { data: options.data }),
    ),
    successOutcome(fifthCharacterTemplatePayload(options.template ?? {})),
    options.preview ?? options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  await startEditing();
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
  await user.click(screen.getByRole('button', { name: editName(message('sheet.pending.save')) }));
}

const pendingCard = () => screen.queryByTestId('sheet-pending-card');

function fieldRow(path: string): HTMLElement {
  const row = document.querySelector<HTMLElement>(`[data-field-path="${path}"]`);
  if (row === null) {
    throw new Error(`missing field row ${path}`);
  }
  return row;
}

function levelInput() {
  return within(fieldRow('identity.level')).getByRole('textbox');
}

async function setLevel(user: ReturnType<typeof userEvent.setup>, level: string) {
  const input = levelInput();
  await user.clear(input);
  await user.type(input, level);
}

function headerEditor(label: string) {
  return within(screen.getByTestId('sheet-name')).getByRole('textbox', { name: label });
}

describe('preset and generic sheets', { timeout: 15_000 }, () => {
  it('Edge: a template without a preset keeps the generic form and offers no mode toggle', async () => {
    await openPlain();

    expect(screen.getByRole('textbox', { name: 'Level' })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Play mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit mode' })).not.toBeInTheDocument();
  });

  it('Edge: a preset sheet draws the preset with no generic Section and no mode toggle', async () => {
    await openFifth();

    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(document.querySelector('[data-section-key]')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Play mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit mode' })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: editName(message('arrange.start')) }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.pinnedOnly')) }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: the generic form keeps Arrange and the pinned filter', async () => {
    await openPlain();

    expect(
      screen.getByRole('button', { name: editName(message('arrange.start')) }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: editName(message('sheet.pinnedOnly')) }),
    ).toBeInTheDocument();
    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });
});

describe('the inline edit control', () => {
  it('Edge: Escape cancels the edit and leaves no pending change', async () => {
    const { fake, user } = await openFifth();

    await user.click(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    );
    const input = headerEditor('Name');
    await user.clear(input);
    await user.type(input, 'Brenna{Escape}');

    expect(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    ).toHaveTextContent('Brenna');
    expect(pendingCard()).not.toBeInTheDocument();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: blur commits the typed value', async () => {
    const { user } = await openFifth();

    await user.click(screen.getByRole('button', { name: editName('Class') }));
    const input = screen.getByRole('textbox', { name: 'Class' });
    await user.clear(input);
    await user.type(input, 'Rogue');
    await user.tab();

    expect(screen.getByRole('button', { name: editName('Class') })).toHaveTextContent('Rogue');
    expect(
      screen.getByRole('button', { name: editName(message('sheet.pending.save')) }),
    ).toBeEnabled();
  });

  it('Edge: committing an empty value writes the empty value and shows the Add copy', async () => {
    const { fake, user } = await openFifth();

    const input = within(fieldRow('identity.subclass')).getByRole('textbox');
    await user.clear(input);
    await user.keyboard('{Enter}');

    expect(input).toHaveValue('');
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { subclass: '' } } }));
  });

  it('Happy path: Enter commits the name, which writes identity.name only', async () => {
    const { fake, user } = await openFifth();

    await user.click(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    );
    const input = headerEditor('Name');
    expect(input).toHaveValue('Brenna');
    await user.clear(input);
    await user.type(input, 'Rune{Enter}');

    expect(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    ).toHaveTextContent('Rune');
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { name: 'Rune' } } }));
  });
});

describe('the pending changes card', () => {
  it('Edge: a sheet with no edit draws no card and reserves no bottom space', async () => {
    await openFifth();

    const sheetRoot = screen.getByTestId('sheet-play');
    expect(pendingCard()).not.toBeInTheDocument();
    expect(sheetRoot).not.toHaveClass(PENDING_CARD_SPACE_CLASS);
    expect(sheetRoot.style.getPropertyValue(PENDING_CARD_SPACE_PROPERTY)).toBe('');
  });

  it('Edge: the card is a named region with one separate polite announcement slot', async () => {
    const { user } = await openFifth();

    await setLevel(user, '6');

    const card = screen.getByTestId('sheet-pending-card');
    expect(card).toHaveAttribute('aria-label', message('sheet.pendingEdits'));
    expect(card).not.toHaveAttribute('aria-live');
    expect(within(card).getByTestId('sheet-pending-announcement')).toHaveAttribute(
      'aria-live',
      'polite',
    );
  });

  it('Edge: editing a value back to its stored value removes the card', async () => {
    const { fake, user } = await openFifth();

    await setLevel(user, '6');
    expect(pendingCard()).toBeInTheDocument();

    await setLevel(user, '5');

    expect(pendingCard()).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.pending.save')) }),
    ).not.toBeInTheDocument();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: clearing a text field shows the empty value in the summary', async () => {
    const { user } = await openFifth();

    const input = within(fieldRow('identity.subclass')).getByRole('textbox');
    await user.clear(input);
    await user.keyboard('{Enter}');

    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      `Subclass Hunter → ${message('sheet.pending.emptyValue')}`,
    );
  });

  it('Error: a failed save keeps the card and shows the mapped error', async () => {
    const { user } = await openFifth({
      save: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });
    await setLevel(user, '6');

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: editName(message('sheet.pending.save')),
      }),
    );

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(pendingCard()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 5 → 6');
    expect(
      screen.getByRole('button', { name: editName(message('sheet.pending.save')) }),
    ).toBeEnabled();
  });

  it('Error: a failed preview keeps the card and shows the mapped error', async () => {
    const { fake, user } = await openFifth({
      preview: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });
    await setLevel(user, '6');

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: editName(message('sheet.pending.preview')),
      }),
    );

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-preview-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(pendingCard()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 5 → 6');
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: two edits list each field with its old and new value', async () => {
    const { user } = await openFifth();

    await user.click(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    );
    const input = headerEditor('Name');
    await user.clear(input);
    await user.type(input, 'Rune{Enter}');
    await setLevel(user, '6');

    const card = screen.getByTestId('sheet-pending-card');
    const changes = within(card).getByTestId('sheet-pending-changes');
    expect(changes).not.toBeVisible();
    await user.click(within(card).getByRole('button', { expanded: false }));
    expect(within(changes).getAllByRole('listitem')).toHaveLength(2);
    expect(changes).toHaveTextContent('Name Brenna → Rune');
    expect(changes).toHaveTextContent('Level 5 → 6');
    expect(
      within(changes).getByRole('listitem', {
        name: messageWith('sheet.pending.change', {
          label: 'Level',
          before: '5',
          after: '6',
        }),
      }),
    ).toBeInTheDocument();
    expect(card).toHaveTextContent(message('sheet.pending.notSavedYet'));
  });

  it('Happy path: Discard restores the stored value and hides the card', async () => {
    const { fake, user } = await openFifth();
    await setLevel(user, '6');

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: editName(message('sheet.pending.discard')),
      }),
    );

    expect(pendingCard()).not.toBeInTheDocument();
    expect(levelInput()).toHaveValue('5');
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.pending.save')) }),
    ).not.toBeInTheDocument();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: Save sends the delta once and hides the card', async () => {
    const { fake, user } = await openFifth();
    await setLevel(user, '6');

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: editName(message('sheet.pending.save')),
      }),
    );

    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 6 } } }));
    await waitFor(() => expect(pendingCard()).not.toBeInTheDocument());
    expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(1);
  });

  it('Happy path: Preview sends the delta and keeps the card unsaved', async () => {
    const { fake, user } = await openFifth();
    await setLevel(user, '6');

    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: editName(message('sheet.pending.preview')),
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
