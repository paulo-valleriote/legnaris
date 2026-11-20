import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { act } from 'react';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { stripOutOfView } from '../sheet/preset/fantasy/vitals_bar';
import { FANTASY_SHEET_ID, fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { setViewportWidth } from './match_media';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const vitalsCss = readFileSync(
  path.resolve(here, '../sheet/preset/fantasy/vitals_bar.css'),
  'utf8',
);
const statBlockCss = readFileSync(
  path.resolve(here, '../sheet/preset/fantasy/stat_block.css'),
  'utf8',
);

async function openFantasy(width = 390) {
  setViewportWidth(width);
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload()),
    successOutcome(fantasyTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function placeStrip(top: number, height = 240) {
  const strip = screen.getByTestId('sheet-combat-strip');
  strip.getBoundingClientRect = () =>
    ({ top, bottom: top + height, height, left: 0, right: 360, width: 360 }) as DOMRect;
  fireEvent.scroll(window);
}

function bars() {
  return screen.queryAllByTestId('sheet-vitals-bar');
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

describe('the compact vitals bar', { timeout: 15_000 }, () => {
  it('Edge: a very short sheet keeps the bar hidden while the strip is in view', async () => {
    await openFantasy(390);

    placeStrip(120);
    expect(bars()).toHaveLength(0);
  });

  it('Edge: the bar is never mounted on a desktop sheet', async () => {
    await openFantasy(1280);
    placeStrip(-400);

    expect(bars()).toHaveLength(0);
  });

  it('Happy path: the bar appears once the strip scrolls out and hides when it returns', async () => {
    await openFantasy(390);
    expect(bars()).toHaveLength(0);

    placeStrip(-300);
    const bar = screen.getByTestId('sheet-vitals-bar');
    expect(within(bar).getByTestId('sheet-vitals-bar-ac')).toHaveTextContent('AC');
    expect(within(bar).getByTestId('sheet-vitals-bar-ac')).toHaveTextContent('12');
    expect(within(bar).getByTestId('sheet-vitals-bar-hp')).toHaveTextContent('10 / 20');
    expect(within(bar).getByTestId('sheet-vitals-bar-meter')).toHaveAttribute(
      'data-band',
      'medium',
    );

    placeStrip(40);
    expect(bars()).toHaveLength(0);
  });

  it('Edge: an orientation change re-measures, so a strip back in view hides the bar', async () => {
    await openFantasy(390);
    placeStrip(-300);
    expect(bars()).toHaveLength(1);

    setViewportWidth(844);
    await act(async () => undefined);
    expect(bars()).toHaveLength(0);

    setViewportWidth(390);
    await act(async () => undefined);
    placeStrip(60);
    fireEvent(window, new Event('resize'));
    expect(bars()).toHaveLength(0);
  });

  it('Happy path: the bar opens Damage and Heal and writes the same pending delta', async () => {
    const { fake, user } = await openFantasy(390);
    placeStrip(-300);

    const bar = screen.getByTestId('sheet-vitals-bar');
    await user.click(within(bar).getByRole('button', { name: message('sheet.play.damageOrHeal') }));

    const panel = within(bar).getByTestId('sheet-vitals-bar-panel');
    const amount = within(panel).getByRole('spinbutton', { name: message('sheet.play.amount') });
    await user.type(amount, '4{Enter}');

    expect(within(bar).getByTestId('sheet-vitals-bar-hp')).toHaveTextContent('6 / 20');
    expect(within(bar).getByTestId('sheet-vitals-bar-pending')).toHaveTextContent('10 → 6 (−4)');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Current hit points 10 → 6',
    );
    expect(amount).toHaveValue(null);

    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { hitPoints: 6 } } }));
  });

  it('Edge: Escape closes the bar panel without discarding the pending change', async () => {
    const { user } = await openFantasy(390);
    placeStrip(-300);

    const bar = screen.getByTestId('sheet-vitals-bar');
    await user.click(within(bar).getByRole('button', { name: message('sheet.play.damageOrHeal') }));
    const panel = within(bar).getByTestId('sheet-vitals-bar-panel');
    await user.type(
      within(panel).getByRole('spinbutton', { name: message('sheet.play.amount') }),
      '4',
    );
    await user.click(within(panel).getByRole('button', { name: message('sheet.play.damage') }));

    await user.keyboard('{Escape}');

    expect(within(bar).queryByTestId('sheet-vitals-bar-panel')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Current hit points 10 → 6',
    );
  });

  it('Edge: an empty amount in the bar writes nothing', async () => {
    const { fake, user } = await openFantasy(390);
    placeStrip(-300);

    const bar = screen.getByTestId('sheet-vitals-bar');
    await user.click(within(bar).getByRole('button', { name: message('sheet.play.damageOrHeal') }));
    const panel = within(bar).getByTestId('sheet-vitals-bar-panel');
    await user.click(within(panel).getByRole('button', { name: message('sheet.play.damage') }));

    expect(within(bar).getByTestId('sheet-vitals-bar-hp')).toHaveTextContent('10 / 20');
    expect(within(bar).queryByTestId('sheet-vitals-bar-pending')).not.toBeInTheDocument();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: the bar spells out a pending change made before it appeared', async () => {
    const { user } = await openFantasy(390);
    const amount = screen.getByRole('spinbutton', { name: message('sheet.play.amount') });
    await user.type(amount, '3{Enter}');

    placeStrip(-300);

    const bar = screen.getByTestId('sheet-vitals-bar');
    expect(within(bar).getByTestId('sheet-vitals-bar-pending')).toHaveTextContent('10 → 7 (−3)');
    expect(within(bar).getByTestId('sheet-vitals-bar-pending')).toHaveAttribute(
      'data-direction',
      'damage',
    );
  });

  it('Edge: a pending heal reaches the bar with its own tone', async () => {
    const { user } = await openFantasy(390);
    const amount = screen.getByRole('spinbutton', { name: message('sheet.play.amount') });
    await user.type(amount, '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    placeStrip(-300);

    const bar = screen.getByTestId('sheet-vitals-bar');
    expect(within(bar).getByTestId('sheet-vitals-bar-pending')).toHaveTextContent('10 → 15 (+5)');
    expect(within(bar).getByTestId('sheet-vitals-bar-pending')).toHaveAttribute(
      'data-direction',
      'heal',
    );
  });

  it('Edge: a focused field never lands under the bar', () => {
    expect(vitalsCss).toMatch(
      /html:has\(\.lg-vitals-bar\)\s*\{[^}]*scroll-padding-block-start:\s*calc\(/,
    );
  });
});

describe('the compact vitals bar styles', () => {
  it('Edge: the bar pins under the top safe-area inset', () => {
    expect(vitalsCss).toMatch(
      /\.lg-vitals-bar\s*\{[^}]*inset-block-start:\s*env\(safe-area-inset-top/,
    );
  });

  it('Edge: the bar keeps 44px actions and a thin HP bar', () => {
    expect(vitalsCss).toMatch(
      /\.lg-vitals-bar__toggle\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    expect(vitalsCss).toMatch(
      /\.lg-vitals-bar__panel > \.lg-button\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    expect(vitalsCss).toMatch(/\.lg-vitals-bar__bar\s*\{[^}]*block-size:\s*4px/);
  });

  it('Edge: reduced motion drops the bar animation and the fill transition', () => {
    const reduced = vitalsCss.slice(vitalsCss.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(reduced).toMatch(/\.lg-vitals-bar\s*\{[^}]*animation:\s*none/);
    expect(reduced).toMatch(/\.lg-vitals-bar__fill\s*\{[^}]*transition:\s*none/);
  });

  it('Edge: the stat block band is sticky at every width and clears the safe area', () => {
    expect(statBlockCss).toMatch(/\.lg-stat-block__combat\s*\{[^}]*position:\s*sticky/);
    expect(statBlockCss).toMatch(
      /\.lg-stat-block__combat\s*\{[^}]*inset-block-start:\s*env\(safe-area-inset-top/,
    );
  });
});

describe('the vitals anchor measurement', () => {
  it('Edge: a missing or zero-height anchor never shows the bar', () => {
    expect(stripOutOfView(undefined)).toBe(false);
    expect(stripOutOfView({ height: 0, bottom: -10 })).toBe(false);
  });

  it('Edge: the bar shows only once the anchor has passed the top edge', () => {
    expect(stripOutOfView({ height: 200, bottom: 1 })).toBe(false);
    expect(stripOutOfView({ height: 200, bottom: 0 })).toBe(true);
    expect(stripOutOfView({ height: 200, bottom: -40 })).toBe(true);
  });
});
