import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import GameMoments, { HeroSheet } from '../landing/game_moments';

const here = path.dirname(fileURLToPath(import.meta.url));
const landingDir = path.resolve(here, '../landing');

function source(file: string): string {
  return readFileSync(path.resolve(landingDir, file), 'utf8');
}

function stubMatchMedia(matches: boolean): () => void {
  const original = window.matchMedia;
  window.matchMedia = ((query: string) =>
    ({
      matches: query.includes('prefers-reduced-motion') ? matches : false,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    }) as MediaQueryList) as typeof window.matchMedia;
  return () => {
    window.matchMedia = original;
  };
}

function AllMoments() {
  return (
    <>
      <HeroSheet />
      <GameMoments />
    </>
  );
}

function xpCard() {
  return screen.getByTestId('landing-moment-xp');
}

function hpCard() {
  return screen.getByTestId('landing-moment-hp');
}

function slotCard() {
  return screen.getByTestId('landing-moment-slot');
}

describe('the landing game moments', () => {
  it('Happy path: the three moments mount the real Fantasy Play components', () => {
    render(<AllMoments />);

    expect(screen.getByTestId('sheet-xp')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-card')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-spell-slots')).toBeInTheDocument();
  });

  it('Happy path: gaining experience fills the bar, then the next level is taken', async () => {
    const user = userEvent.setup();
    render(<AllMoments />);

    const bar = within(xpCard()).getByRole('progressbar', {
      name: message('sheet.play.experience'),
    });
    expect(bar).toHaveAttribute('aria-valuenow', '1400');

    await user.click(
      within(xpCard()).getByRole('button', {
        name: messageWith('landing.momentXpGain', { count: '1,300' }),
      }),
    );

    expect(bar).toHaveAttribute('aria-valuenow', '2700');
    expect(within(xpCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentXpAnnounce', { count: '2,700' }),
    );
    expect(within(xpCard()).getByRole('status')).toHaveFocus();

    await user.click(
      within(xpCard()).getByRole('button', {
        name: messageWith('landing.momentXpLevelUp', { count: 4 }),
      }),
    );

    expect(within(xpCard()).getByTestId('landing-moment-xp-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 4 }),
    );
    expect(within(xpCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentXpValue', { count: 4, total: '2,700' }),
    );
  });

  it('Happy path: taking damage lowers the hit points and announces the total', async () => {
    const user = userEvent.setup();
    render(<AllMoments />);

    await user.click(
      within(hpCard()).getByRole('button', {
        name: messageWith('sheet.play.damageAmount', { count: 5 }),
      }),
    );
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent(/19\s*\/\s*30/);
    expect(within(hpCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentHpAnnounce', { count: 19, total: 30 }),
    );

    await user.click(
      within(hpCard()).getByRole('button', {
        name: messageWith('sheet.play.damageAmount', { count: 5 }),
      }),
    );
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent(/14\s*\/\s*30/);
  });

  it('Happy path: spending a focus slot updates the pips and the remaining count', async () => {
    const user = userEvent.setup();
    render(<AllMoments />);

    await user.click(
      within(slotCard()).getByRole('button', { name: message('landing.momentSlotSpend') }),
    );

    expect(screen.getByTestId('sheet-slot-1-remaining')).toHaveTextContent(
      messageWith('sheet.play.slotsLeft', { remaining: 1, total: 2 }),
    );
    expect(
      within(screen.getByTestId('sheet-slot-1')).getAllByRole('button', {
        name: messageWith('sheet.play.slotRecover', { count: 1 }),
      }),
    ).toHaveLength(1);
    expect(within(slotCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentSlotAnnounce', { count: 1 }),
    );
  });

  it('Edge: a spent focus slot cannot be spent again', async () => {
    const user = userEvent.setup();
    render(<AllMoments />);

    const spend = within(slotCard()).getByRole('button', {
      name: message('landing.momentSlotSpend'),
    });
    await user.click(spend);
    await user.click(spend);

    expect(screen.getByTestId('sheet-slot-1-remaining')).toHaveTextContent(
      messageWith('sheet.play.slotsLeft', { remaining: 0, total: 2 }),
    );
    expect(spend).toBeDisabled();
    expect(within(slotCard()).getByRole('status')).toHaveFocus();
  });

  it('Happy path: an action replays a short flash when motion is allowed', async () => {
    const restore = stubMatchMedia(false);
    const user = userEvent.setup();
    render(<AllMoments />);

    await user.click(
      within(hpCard()).getByRole('button', {
        name: messageWith('sheet.play.damageAmount', { count: 5 }),
      }),
    );

    expect(hpCard().querySelector('.lg-moment__flash')).not.toBeNull();
    restore();
  });

  it('Edge: under prefers-reduced-motion the moments draw no flash', async () => {
    const restore = stubMatchMedia(true);
    const user = userEvent.setup();
    render(<AllMoments />);

    await user.click(
      within(hpCard()).getByRole('button', {
        name: messageWith('sheet.play.damageAmount', { count: 5 }),
      }),
    );

    expect(hpCard().querySelector('.lg-moment__flash')).toBeNull();
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent(/19\s*\/\s*30/);
    restore();
  });

  it('Happy path: every moment is a figure with its own name and live result', () => {
    render(<AllMoments />);

    for (const card of [xpCard(), hpCard(), slotCard()]) {
      expect(card).toHaveAccessibleName();
      expect(within(card).getByRole('status')).toHaveAttribute('aria-live', 'polite');
    }
  });
});

describe('the hero sheet and the moments grid', () => {
  it('Happy path: the hero sheet holds the live hit points moment', () => {
    render(<HeroSheet />);

    const hero = screen.getByTestId('landing-hero-sheet');
    expect(within(hero).getByTestId('landing-moment-hp')).toBeInTheDocument();
    expect(within(hero).getByTestId('sheet-hp-card')).toBeInTheDocument();
  });

  it('Edge: the moments grid no longer repeats the hit points moment', () => {
    render(<GameMoments />);

    const grid = screen.getByTestId('landing-game-moments');
    expect(within(grid).queryByTestId('landing-moment-hp')).not.toBeInTheDocument();
    expect(within(grid).getByTestId('landing-moment-xp')).toBeInTheDocument();
    expect(within(grid).getByTestId('landing-moment-slot')).toBeInTheDocument();
  });

  it('Edge: the hero sheet and the grid keep separate demo state', async () => {
    const user = userEvent.setup();
    render(<AllMoments />);

    await user.click(
      within(hpCard()).getByRole('button', {
        name: messageWith('sheet.play.damageAmount', { count: 5 }),
      }),
    );

    expect(screen.getAllByTestId('sheet-hp-value')).toHaveLength(1);
    expect(within(slotCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentSlotAnnounce', { count: 2 }),
    );
  });
});

describe('the landing game moments contract', () => {
  it('Happy path: the static page loads the moments as their own chunk', () => {
    const landing = source('landing_screen.tsx');

    expect(landing).toContain("lazy(() => import('./game_moments'))");
    expect(landing).not.toContain("from './game_moments'");
    expect(landing).toMatch(/<Suspense[\s\S]*fallback=/);
  });

  it('Edge: the moments module composes the real preset components through the demo provider', () => {
    const moments = source('game_moments.tsx');

    expect(moments).toContain("from '../sheet/preset/fantasy/xp_panel'");
    expect(moments).toContain("from '../sheet/preset/fantasy/hp_card'");
    expect(moments).toContain("from '../sheet/preset/fantasy/spells'");
    expect(moments).toContain("from './demo_provider'");
  });

  it('Edge: the cascade demo and the home ad slot are gone', () => {
    const landing = source('landing_screen.tsx');

    expect(landing).not.toContain('CascadeDemo');
    expect(landing).not.toContain('AdSlot');
    expect(existsSync(path.resolve(landingDir, 'cascade_demo.tsx'))).toBe(false);
    expect(existsSync(path.resolve(landingDir, 'cascade.ts'))).toBe(false);
  });

  it('Edge: the page keeps the FAQ and the privacy messaging', () => {
    const landing = source('landing_screen.tsx');

    expect(landing).toContain('landing.faqTitle');
    expect(landing).toContain('landing.footer.privacy');
    expect(landing).toContain('PRIVACY_PATH');
  });
});
