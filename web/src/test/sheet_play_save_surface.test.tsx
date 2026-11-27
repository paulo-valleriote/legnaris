import { startEditing } from './start_editing';
import { editName } from './dice_names';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  PENDING_ANNOUNCE_MS,
  PENDING_CARD_SPACE_CLASS,
  PENDING_CARD_SPACE_PROPERTY,
} from '../sheet/preset/pending_card';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import type { FakeOutcome } from './fake_transport';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const indexHtml = readFileSync(path.resolve(here, '../../index.html'), 'utf8');
const presetCss = readFileSync(path.resolve(here, '../sheet/preset/preset.css'), 'utf8');
const demoCss = readFileSync(path.resolve(here, '../landing/demo_page.css'), 'utf8');

interface OpenOptions {
  readonly save?: FakeOutcome;
}

async function openFifth(options: OpenOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload()),
    successOutcome(fifthCharacterTemplatePayload()),
    options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  await startEditing();
  return { fake, user };
}

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

function card() {
  return screen.queryByTestId('sheet-pending-card');
}

function cardButton(key: 'discard' | 'preview' | 'save') {
  return within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
    name: message(`sheet.pending.${key}`),
  });
}

function footer() {
  return document.querySelector('.lg-sheet__footer');
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

function announcement() {
  return screen.getByTestId('sheet-pending-announcement');
}

interface DeferredFifth {
  readonly transport: Transport;
  readonly requests: HttpRequest[];
  resolve(response: HttpResponse): void;
}

function deferredFifth(): DeferredFifth {
  let settle: ((response: HttpResponse) => void) | undefined;
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    if (request.method === 'PATCH') {
      return new Promise<HttpResponse>((resolve) => {
        settle = resolve;
      });
    }
    if (request.url.includes('/templates/')) {
      return Promise.resolve(successOutcome(fifthCharacterTemplatePayload()));
    }
    return Promise.resolve(successOutcome(fifthCharacterSheetPayload()));
  };
  return {
    transport,
    requests,
    resolve(response) {
      settle?.(response);
    },
  };
}

describe('the preset save surface', () => {
  it('Edge: a preset sheet with no pending changes renders no Preview or Save footer', async () => {
    await openFifth();

    expect(card()).not.toBeInTheDocument();
    expect(footer()).toBeNull();
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.save')) }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.preview')) }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.pending.save')) }),
    ).toBeNull();
  });

  it('Edge: Escape with nothing pending sends no request and changes nothing', async () => {
    const { fake, user } = await openFifth();

    await user.keyboard('{Escape}');

    expect(card()).not.toBeInTheDocument();
    expect(levelInput()).toHaveValue('5');
    expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(0);
  });

  it('Edge: Escape inside an inline edit cancels only that edit and keeps other pending changes', async () => {
    const { user } = await openFifth();
    await setLevel(user, '6');
    await user.click(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    );
    const input = within(screen.getByTestId('sheet-name')).getByRole('textbox', { name: 'Name' });
    await user.clear(input);
    await user.type(input, 'Rune{Escape}');

    expect(
      within(screen.getByTestId('sheet-name')).queryByRole('textbox', { name: 'Name' }),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId('sheet-name')).getByRole('button', { name: editName('Name') }),
    ).toHaveTextContent('Brenna');
    expect(card()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 5 → 6');
  });

  it('Edge: Escape outside inputs keeps the pending changes', async () => {
    const { fake, user } = await openFifth();
    await setLevel(user, '6');

    act(() => {
      (document.activeElement as HTMLElement | null)?.blur();
    });
    await user.keyboard('{Escape}');

    expect(card()).toBeInTheDocument();
    expect(levelInput()).toHaveValue('6');
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: Ctrl+S with nothing pending sends no request and blocks the browser save dialog', async () => {
    const { fake } = await openFifth();

    const event = new KeyboardEvent('keydown', {
      key: 's',
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });
    const allowed = document.dispatchEvent(event);

    expect(allowed).toBe(false);
    expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(0);
    expect(card()).not.toBeInTheDocument();
  });

  it('Edge: the card renders no table-sync pill', async () => {
    const { user } = await openFifth();
    await setLevel(user, '6');

    expect(screen.getByTestId('sheet-pending-card')).not.toHaveTextContent('Table sync soon');
    expect(screen.queryByTestId('sheet-pending-sync')).not.toBeInTheDocument();
  });

  it('Happy path: the visible card reserves its measured height on the sheet root', async () => {
    const { user } = await openFifth();
    await setLevel(user, '6');

    const root = screen.getByTestId('sheet-play');
    const cardElement = screen.getByTestId('sheet-pending-card');
    expect(root).toHaveClass(PENDING_CARD_SPACE_CLASS);

    vi.spyOn(cardElement, 'getBoundingClientRect').mockReturnValue({
      height: 120,
      bottom: window.innerHeight - 80,
    } as DOMRect);
    act(() => {
      window.dispatchEvent(new Event('resize'));
    });

    const space = Number.parseFloat(root.style.getPropertyValue(PENDING_CARD_SPACE_PROPERTY));
    expect(Number.isFinite(space)).toBe(true);
    expect(space).toBeGreaterThanOrEqual(120);
  });

  it('Edge: hiding the card releases the reserved bottom space', async () => {
    const { user } = await openFifth();
    await setLevel(user, '6');

    const root = screen.getByTestId('sheet-play');
    expect(root).toHaveClass(PENDING_CARD_SPACE_CLASS);

    await user.click(cardButton('discard'));

    expect(root).not.toHaveClass(PENDING_CARD_SPACE_CLASS);
    expect(root.style.getPropertyValue(PENDING_CARD_SPACE_PROPERTY)).toBe('');
  });

  it('Edge: rapid edits settle into one debounced announcement, not one per edit', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { user } = await openFifth();
      await setLevel(user, '6');

      expect(announcement()).toHaveTextContent('');
      await setLevel(user, '7');
      expect(announcement()).toHaveTextContent('');

      await act(async () => {
        vi.advanceTimersByTime(PENDING_ANNOUNCE_MS);
      });

      expect(announcement()).toHaveTextContent(
        messageWith('sheet.pending.announce', { summary: 'Level 5 → 7' }),
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it('Error: a failed save keeps the bar and shows the typed error', async () => {
    const { user } = await openFifth({
      save: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });
    await setLevel(user, '6');

    await user.click(cardButton('save'));

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(card()).toBeInTheDocument();
    expect(footer()).toBeNull();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 5 → 6');
  });

  it('Happy path: pending changes render only the bar, with Discard, Preview and Save', async () => {
    const { user } = await openFifth();
    await setLevel(user, '6');

    expect(card()).toBeInTheDocument();
    expect(footer()).toBeNull();
    expect(cardButton('discard')).toBeInTheDocument();
    expect(cardButton('preview')).toBeInTheDocument();
    expect(cardButton('save')).toBeEnabled();
    expect(
      screen.queryByRole('button', { name: editName(message('sheet.preview')) }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: Ctrl+S saves the pending changes', async () => {
    const { fake, user } = await openFifth();
    await setLevel(user, '6');

    await user.keyboard('{Control>}s{/Control}');

    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 6 } } }));
    await waitFor(() => expect(card()).not.toBeInTheDocument());
  });

  it('Happy path: Ctrl+S shows the same Saved confirmation as the card Save button', async () => {
    const { user } = await openFifth();
    await setLevel(user, '6');

    await user.keyboard('{Control>}s{/Control}');

    const saved = await screen.findByTestId('sheet-pending-saved');
    expect(saved).toHaveTextContent(message('sheet.saved'));
    expect(within(saved).getByRole('status')).toHaveAttribute('aria-live', 'polite');
    expect(screen.queryByTestId('sheet-saved-announcement')).not.toBeInTheDocument();
    expect(levelInput()).toHaveFocus();
  });

  it('Error: a failed save shows the error and no Saved confirmation', async () => {
    const { user } = await openFifth({ save: errorOutcome('INTERNAL_ERROR', 'boom', 500) });
    await setLevel(user, '6');

    await user.click(cardButton('save'));

    expect(await screen.findByTestId('sheet-pending-error')).toHaveTextContent(
      message('api.error.serverError'),
    );
    expect(screen.queryByTestId('sheet-pending-saved')).not.toBeInTheDocument();
    expect(card()).toBeInTheDocument();
  });

  it('Happy path: an edit made during the save stays pending without a Saved confirmation', async () => {
    const deferred = deferredFifth();
    const user = userEvent.setup();
    renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: deferred.transport }),
    });
    await screen.findByTestId('sheet-name');
    await startEditing();

    await setLevel(user, '6');
    await user.click(cardButton('save'));
    await setLevel(user, '7');
    deferred.resolve(successOutcome({}));

    await waitFor(() => expect(cardButton('save')).toBeEnabled());
    expect(card()).toHaveTextContent(message('sheet.changeCountOne'));
    expect(screen.queryByTestId('sheet-pending-saved')).not.toBeInTheDocument();
    expect(levelInput()).toHaveValue('7');
  });

  it('Edge: the change list starts closed and the count names how many are pending', async () => {
    const { user } = await openFifth();

    await setLevel(user, '6');

    const toggle = within(card() as HTMLElement).getByRole('button', { expanded: false });
    expect(toggle).toHaveTextContent(message('sheet.changeCountOne'));
    expect(screen.getByTestId('sheet-pending-changes')).not.toBeVisible();

    await user.click(toggle);

    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('sheet-pending-changes')).toBeVisible();

    await user.click(toggle);

    expect(screen.getByTestId('sheet-pending-changes')).not.toBeVisible();
  });

  it('Edge: the document declares viewport-fit=cover for the safe-area insets', () => {
    expect(indexHtml).toMatch(/<meta\s+name="viewport"[^>]*viewport-fit=cover/);
  });

  it('Edge: the bar clears the home indicator and reserves its own height in the page', () => {
    expect(presetCss).toMatch(
      /\.lg-pending-card\s*\{[^}]*bottom:\s*calc\([^;]*env\(safe-area-inset-bottom/,
    );
    expect(presetCss).toMatch(
      /\.lg-preset--pending-card\s*\{[^}]*padding-block-end:\s*var\(--lg-pending-card-space/,
    );
  });

  it('Edge: on a phone the demo card replaces the sticky sign-up bar at the bottom edge', () => {
    const phone = demoCss.slice(demoCss.indexOf('@media (max-width: 39.99rem)'));
    expect(phone).toMatch(/\.lg-demo\s+\.lg-pending-card\s*\{[^}]*bottom:\s*max\(/);
    expect(demoCss).toMatch(
      /\.lg-demo:has\(\.lg-pending-card\)\s+\.lg-landing__sticky\s*\{[^}]*display:\s*none/,
    );
  });

  it('Edge: the bar actions are 44px tall on a phone and its slide-in respects reduced motion', () => {
    const phone = presetCss.slice(presetCss.indexOf('@media (max-width: 40rem)'));
    expect(phone).toMatch(
      /\.lg-pending-card__actions\s+\.lg-button\s*\{[^}]*min-height:\s*var\(--lg-size-control\)/,
    );
    const reduced = presetCss.slice(presetCss.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(reduced).toMatch(/\.lg-pending-card\s*\{[^}]*animation:\s*none/);
    expect(presetCss).toMatch(/\.lg-pending-card\s*\{[^}]*animation:\s*lg-pending-card-in/);
  });
});
