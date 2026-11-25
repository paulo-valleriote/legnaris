import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { valueAtPath } from '../api/models/sheet';
import type { FieldNode } from '../api/models/template';
import { message, messageWith } from '../l10n/strings';
import { createDemoEditStore } from '../landing/demo_store';
import { FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from '../landing/fifth_demo_character';
import GameMoments, { HeroSheet } from '../landing/game_moments';
import { longRestCascade } from '../landing/moment_demo';
import { templateDerivation } from '../landing/template_derivation';

const here = path.dirname(fileURLToPath(import.meta.url));
const landingDir = path.resolve(here, '../landing');

function source(file: string): string {
  return readFileSync(path.resolve(landingDir, file), 'utf8');
}

function listTarget(target: string): { list: string; field: string } | undefined {
  const segments = target.split('.');
  let nodes: Record<string, FieldNode> = FIFTH_DEMO_TEMPLATE.fields;
  const walked: string[] = [];
  for (let index = 0; index < segments.length; index += 1) {
    const segment = segments[index];
    const node = nodes[segment];
    if (node === undefined) {
      return undefined;
    }
    if (node.kind === 'list') {
      return {
        list: [...walked, segment].join('.'),
        field: segments.slice(index + 1).join('.'),
      };
    }
    if (node.kind !== 'group') {
      return undefined;
    }
    nodes = node.children;
    walked.push(segment);
  }
  return undefined;
}

function AllMoments() {
  return (
    <>
      <HeroSheet />
      <GameMoments />
    </>
  );
}

function hero() {
  return screen.getByTestId('landing-hero-sheet');
}

function grid() {
  return screen.getByTestId('landing-game-moments');
}

function heroHpCard() {
  return within(hero()).getByTestId('landing-moment-hp');
}

function gridHpCard() {
  return within(grid()).getByTestId('landing-moment-hp');
}

function slotCard() {
  return within(grid()).getByTestId('landing-moment-slot');
}

function restCard() {
  return within(grid()).getByTestId('landing-moment-rest');
}

function hitPointsMeter(card: HTMLElement) {
  return within(card).getByRole('meter', { name: message('sheet.play.hitPoints') });
}

async function takeDamage(
  user: ReturnType<typeof userEvent.setup>,
  card: HTMLElement,
  amount: number,
) {
  const field = within(card).getByLabelText(message('sheet.play.amount'));
  await user.clear(field);
  await user.type(field, String(amount));
  await user.click(within(card).getByRole('button', { name: message('sheet.play.damage') }));
}

function slotGroup(card: HTMLElement, count: number) {
  return within(card).getByRole('group', {
    name: messageWith('preset.pips.group', {
      label: 'Level 1 spell slots spent',
      count,
      max: 4,
    }),
  });
}

describe('the landing game moments', () => {
  it('Happy path: the moments section shows damage, a spent slot and a long rest', () => {
    render(<GameMoments />);

    expect(within(grid()).getByTestId('landing-moment-hp')).toBeInTheDocument();
    expect(within(grid()).getByTestId('landing-moment-slot')).toBeInTheDocument();
    expect(within(grid()).getByTestId('landing-moment-rest')).toBeInTheDocument();
    expect(within(grid()).queryByTestId('landing-moment-xp')).not.toBeInTheDocument();
  });

  it('Happy path: the moments draw the 5E demo character and its prepared values', () => {
    render(<GameMoments />);

    expect(hitPointsMeter(gridHpCard())).toHaveAttribute('aria-valuenow', '24');
    expect(hitPointsMeter(gridHpCard())).toHaveAttribute('aria-valuemax', '24');
    expect(slotGroup(slotCard(), 1)).toBeInTheDocument();
  });

  it('Happy path: taking damage lowers the hit points and announces the total', async () => {
    const user = userEvent.setup();
    render(<GameMoments />);

    await takeDamage(user, gridHpCard(), 5);

    expect(hitPointsMeter(gridHpCard())).toHaveAttribute('aria-valuenow', '19');
    expect(within(gridHpCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentHpAnnounce', { count: 19, total: 24 }),
    );
  });

  it('Happy path: spending a spell slot updates the pips and the remaining count', async () => {
    const user = userEvent.setup();
    render(<GameMoments />);

    await user.click(
      within(slotCard()).getByRole('button', { name: message('landing.momentSlotSpend') }),
    );

    expect(slotGroup(slotCard(), 2)).toBeInTheDocument();
    expect(within(slotCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentSlotAnnounce', { count: 2 }),
    );
  });

  it('Edge: the last slot cannot be spent twice', async () => {
    const user = userEvent.setup();
    render(<GameMoments />);

    const spend = within(slotCard()).getByRole('button', {
      name: message('landing.momentSlotSpend'),
    });
    await user.click(spend);
    await user.click(spend);
    await user.click(spend);

    expect(slotGroup(slotCard(), 4)).toBeInTheDocument();
    expect(spend).toBeDisabled();
    expect(within(slotCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentSlotAnnounce', { count: 0 }),
    );
  });

  it('Happy path: a long rest restores hit points and every spent spell slot', async () => {
    const user = userEvent.setup();
    render(<GameMoments />);

    await takeDamage(user, gridHpCard(), 5);

    expect(within(restCard()).getByTestId('landing-moment-rest-hp')).toHaveTextContent(
      messageWith('landing.momentRestHp', { count: 19, total: 24 }),
    );

    await user.click(
      within(restCard()).getByRole('button', { name: message('landing.momentRestAction') }),
    );

    expect(hitPointsMeter(gridHpCard())).toHaveAttribute('aria-valuenow', '24');
    expect(within(restCard()).getByTestId('landing-moment-rest-hp')).toHaveTextContent(
      messageWith('landing.momentRestHp', { count: 24, total: 24 }),
    );
    expect(slotGroup(restCard(), 0)).toBeInTheDocument();
    expect(within(restCard()).getByRole('status')).toHaveTextContent(
      messageWith('landing.momentRestAnnounce', { count: 24, total: 24, slots: 4 }),
    );
    expect(within(restCard()).getByRole('status')).toHaveFocus();
  });

  it('Happy path: every moment is a figure with its own name and live result', () => {
    render(<GameMoments />);

    for (const card of [gridHpCard(), slotCard(), restCard()]) {
      expect(card).toHaveAccessibleName();
      expect(within(card).getByRole('status')).toHaveAttribute('aria-live', 'polite');
    }
  });
});

describe('the hero sheet and the moments grid', () => {
  it('Happy path: the hero sheet holds the live 5E hit points moment', () => {
    render(<HeroSheet />);

    expect(within(hero()).getByTestId('landing-moment-hp')).toBeInTheDocument();
    expect(within(hero()).getByTestId('compose-hit-points')).toBeInTheDocument();
    expect(hitPointsMeter(heroHpCard())).toHaveAttribute('aria-valuemax', '24');
  });

  it('Edge: the hero sheet and the grid keep separate demo state', async () => {
    const user = userEvent.setup();
    render(<AllMoments />);

    await takeDamage(user, heroHpCard(), 5);

    expect(hitPointsMeter(heroHpCard())).toHaveAttribute('aria-valuenow', '19');
    expect(hitPointsMeter(gridHpCard())).toHaveAttribute('aria-valuenow', '24');
  });
});

describe('the long rest cascade', () => {
  it('Happy path: it restores hit points and clears every spell slot', () => {
    const cascade = longRestCascade(FIFTH_DEMO_SHEET.data);
    const spellcasting = cascade.spellcasting as Record<string, unknown>;

    expect(cascade.combat).toEqual({ hp: { current: 24 } });
    for (let level = 1; level <= 9; level += 1) {
      expect(spellcasting[`slot${level}`]).toEqual({ spent: 0 });
    }
  });

  it('Happy path: it covers every long_rest reset target the template declares', () => {
    const action = FIFTH_DEMO_TEMPLATE.actions?.find((entry) => entry.id === 'long_rest');
    expect(action).toBeDefined();
    const cascade = longRestCascade(FIFTH_DEMO_SHEET.data);

    for (const assignment of action?.assignments ?? []) {
      const list = listTarget(assignment.target);
      if (list === undefined) {
        expect(valueAtPath(cascade, assignment.target)).toBeDefined();
        continue;
      }
      const records = valueAtPath(FIFTH_DEMO_SHEET.data, list.list) as Record<string, unknown>;
      expect(Object.keys(records).length).toBeGreaterThan(0);
      for (const id of Object.keys(records)) {
        expect(valueAtPath(cascade, `${list.list}.${id}.${list.field}`)).toBeDefined();
      }
    }
  });

  it('Happy path: the demo store recalculates the remaining slots after a rest', () => {
    const store = createDemoEditStore(
      FIFTH_DEMO_TEMPLATE,
      FIFTH_DEMO_SHEET,
      templateDerivation(FIFTH_DEMO_TEMPLATE),
    );

    store.applyReset(longRestCascade(store.getState().data));

    expect(valueAtPath(store.getState().data, 'spellcasting.slot1.spent')).toBe(0);
    expect(valueAtPath(store.getState().data, 'spellcasting.slot1.remaining')).toBe(4);
  });

  it('Edge: a missing hit point maximum reads as zero', () => {
    const cascade = longRestCascade({});

    expect(cascade.combat).toEqual({ hp: { current: 0 } });
  });

  it('Edge: the cascade leaves the given data untouched', () => {
    const data = FIFTH_DEMO_SHEET.data;
    const before = JSON.stringify(data);

    longRestCascade(data);

    expect(JSON.stringify(data)).toBe(before);
  });
});

describe('the landing game moments contract', () => {
  it('Happy path: the static page loads the moments as their own chunk', () => {
    const landing = source('landing_screen.tsx');

    expect(landing).toContain("lazy(() => import('./game_moments'))");
    expect(landing).not.toContain("from './game_moments'");
    expect(landing).toMatch(/<Suspense[\s\S]*fallback=/);
  });

  it('Happy path: the moments compose the real 5E preset parts over the prepared demo', () => {
    const moments = source('game_moments.tsx');

    expect(moments).toContain("from '../sheet/preset/compose/hit_points_track'");
    expect(moments).toContain("from '../sheet/preset/compose/pips'");
    expect(moments).toContain("from './moment_demo'");
    expect(moments).toContain("from './demo_provider'");
    expect(moments).not.toContain('sheet/preset/fantasy');
  });

  it('Edge: the moment demo is the prepared 5E character, not a synthetic template', () => {
    const demo = source('moment_demo.ts');

    expect(demo).toContain("from './fifth_demo_character'");
    expect(demo).not.toContain('landing_moments_v1');
    expect(demo).not.toContain('MOMENT_TEMPLATE_ID');
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
