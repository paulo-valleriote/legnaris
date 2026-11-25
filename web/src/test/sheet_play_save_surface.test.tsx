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
import { FANTASY_SHEET_ID, fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
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

async function openFantasy(options: OpenOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload()),
    successOutcome(fantasyTemplatePayload()),
    options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function levelStep(name: 'increase' | 'decrease') {
  return screen.getByRole('button', { name: messageWith(`engine.${name}`, { label: 'Level' }) });
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

describe('the Play save surface', () => {
  it('Edge: Play with no pending changes renders no Preview or Save footer', async () => {
    await openFantasy();

    expect(card()).not.toBeInTheDocument();
    expect(footer()).toBeNull();
    expect(screen.queryByRole('button', { name: message('sheet.save') })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.preview') }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('sheet.pending.save') })).toBeNull();
  });

  it('Edge: Escape with nothing pending sends no request and changes nothing', async () => {
    const { fake, user } = await openFantasy();

    await user.keyboard('{Escape}');

    expect(card()).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 3 }),
    );
    expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(0);
  });

  it('Edge: Escape inside an inline edit cancels only that edit and keeps other pending changes', async () => {
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));
    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(input);
    await user.type(input, 'Brenna{Escape}');

    expect(screen.queryByRole('textbox', { name: 'Character name' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(card()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 3 → 4');
  });

  it('Edge: Ctrl+S with nothing pending sends no request and blocks the browser save dialog', async () => {
    const { fake } = await openFantasy();

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
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));

    expect(screen.getByTestId('sheet-pending-card')).not.toHaveTextContent('Table sync soon');
    expect(screen.queryByTestId('sheet-pending-sync')).not.toBeInTheDocument();
  });

  it('Happy path: the visible card reserves its measured height on the sheet root', async () => {
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));

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
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));

    const root = screen.getByTestId('sheet-play');
    expect(root).toHaveClass(PENDING_CARD_SPACE_CLASS);

    await user.click(cardButton('discard'));

    expect(root).not.toHaveClass(PENDING_CARD_SPACE_CLASS);
    expect(root.style.getPropertyValue(PENDING_CARD_SPACE_PROPERTY)).toBe('');
  });

  it('Edge: rapid edits settle into one debounced announcement, not one per edit', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { user } = await openFantasy();
      await user.click(levelStep('increase'));

      expect(announcement()).toHaveTextContent('');
      await user.click(levelStep('increase'));
      expect(announcement()).toHaveTextContent('');

      await act(async () => {
        vi.advanceTimersByTime(PENDING_ANNOUNCE_MS);
      });

      expect(announcement()).toHaveTextContent(
        messageWith('sheet.pending.announce', { summary: 'Level 3 → 5' }),
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it('Error: a failed save keeps the bar and shows the typed error', async () => {
    const { user } = await openFantasy({
      save: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });
    await user.click(levelStep('increase'));

    await user.click(cardButton('save'));

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(card()).toBeInTheDocument();
    expect(footer()).toBeNull();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Level 3 → 4');
  });

  it('Happy path: pending changes render only the bar, with Discard, Preview and Save', async () => {
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));

    expect(card()).toBeInTheDocument();
    expect(footer()).toBeNull();
    expect(cardButton('discard')).toBeInTheDocument();
    expect(cardButton('preview')).toBeInTheDocument();
    expect(cardButton('save')).toBeEnabled();
    expect(
      screen.queryByRole('button', { name: message('sheet.preview') }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: Escape outside inputs discards the pending changes', async () => {
    const { fake, user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.keyboard('{Escape}');

    expect(card()).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 3 }),
    );
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: Ctrl+S saves the pending changes', async () => {
    const { fake, user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.keyboard('{Control>}s{/Control}');

    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 4 } } }));
    await waitFor(() => expect(card()).not.toBeInTheDocument());
  });

  it('Happy path: Edit mode keeps the footer and hides the pending bar', async () => {
    const { user } = await openFantasy();
    await user.click(levelStep('increase'));

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(footer()).not.toBeNull();
    expect(card()).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
    expect(screen.getByRole('button', { name: message('sheet.preview') })).toBeEnabled();
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

  it('Edge: on a phone the demo card stacks above the sticky sign-up bar', () => {
    const phone = demoCss.slice(demoCss.indexOf('@media (max-width: 39.99rem)'));
    expect(phone).toMatch(/\.lg-demo\s+\.lg-pending-card\s*\{[^}]*bottom:\s*calc\(/);
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
