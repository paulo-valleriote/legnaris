import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  LEVEL_PATH,
  parseXpGain,
  XP_PATH,
  XP_THRESHOLDS,
  xpProgress,
} from '../sheet/preset/fantasy/xp_progress';
import {
  FANTASY_SHEET_ID,
  fantasySheetData,
  fantasySheetPayload,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

async function openXp(identity: Record<string, unknown> = {}) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data: fantasySheetData(identity) })),
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

function xpBar() {
  return screen.getByRole('progressbar', { name: message('sheet.play.experience') });
}

function xpToggle() {
  return screen.getByRole('button', { name: new RegExp(message('sheet.play.experience')) });
}

function gainInput() {
  return screen.getByRole('spinbutton', { name: message('sheet.play.xpGain') });
}

async function openPanel(user: ReturnType<typeof userEvent.setup>) {
  await user.click(xpBar());
  return screen.getByTestId('sheet-xp-panel');
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

describe('the experience threshold table', () => {
  it('Edge: the table holds one floor per level, from zero to the level 20 cap', () => {
    expect(XP_THRESHOLDS).toHaveLength(20);
    expect(XP_THRESHOLDS[0]).toBe(0);
    expect(XP_THRESHOLDS[2]).toBe(900);
    expect(XP_THRESHOLDS[19]).toBe(355000);
    for (let index = 1; index < XP_THRESHOLDS.length; index += 1) {
      expect(XP_THRESHOLDS[index]).toBeGreaterThan(XP_THRESHOLDS[index - 1]);
    }
  });

  it('Edge: a level outside the table clamps to the nearest legal level', () => {
    expect(xpProgress(0, 0).level).toBe(1);
    expect(xpProgress(-4, 0).level).toBe(1);
    expect(xpProgress(21, 355000).level).toBe(20);
    expect(xpProgress(Number.NaN, 0).level).toBe(1);
  });

  it('Edge: a missing, negative or non-finite total reads as zero', () => {
    expect(xpProgress(3, undefined).xp).toBe(0);
    expect(xpProgress(3, null).xp).toBe(0);
    expect(xpProgress(3, -40).xp).toBe(0);
    expect(xpProgress(3, Number.NaN).xp).toBe(0);
    expect(xpProgress(3, Number.POSITIVE_INFINITY).xp).toBe(0);
  });

  it('Edge: experience below the level floor leaves the bar empty at the floor', () => {
    const progress = xpProgress(5, 1000);

    expect(progress.floor).toBe(6500);
    expect(progress.value).toBe(6500);
    expect(progress.percent).toBe(0);
    expect(progress.canLevelUp).toBe(false);
  });

  it('Edge: experience exactly at the threshold fills the bar and offers the next level', () => {
    const progress = xpProgress(3, 2700);

    expect(progress.nextLevel).toBe(4);
    expect(progress.nextThreshold).toBe(2700);
    expect(progress.value).toBe(2700);
    expect(progress.percent).toBe(100);
    expect(progress.canLevelUp).toBe(true);
  });

  it('Edge: level 20 fills the bar and never offers a next level', () => {
    const atCap = xpProgress(20, 355000);
    const aboveCap = xpProgress(20, 900000);
    const belowCap = xpProgress(20, 0);

    expect(atCap.nextLevel).toBeNull();
    expect(atCap.nextThreshold).toBeNull();
    expect(atCap.canLevelUp).toBe(false);
    expect(atCap.percent).toBe(100);
    expect(aboveCap.percent).toBe(100);
    expect(aboveCap.canLevelUp).toBe(false);
    expect(belowCap.percent).toBe(100);
    expect(belowCap.canLevelUp).toBe(false);
  });

  it('Happy path: level 1 spans zero to 300 and level 3 spans 900 to 2700', () => {
    expect(xpProgress(1, 150)).toMatchObject({
      floor: 0,
      nextThreshold: 300,
      nextLevel: 2,
      value: 150,
      percent: 50,
      canLevelUp: false,
    });
    expect(xpProgress(3, 1800)).toMatchObject({
      floor: 900,
      nextThreshold: 2700,
      nextLevel: 4,
      value: 1800,
      percent: 50,
      canLevelUp: false,
    });
  });

  it('Edge: experience past the next threshold leaves the bar full and offers the level', () => {
    const progress = xpProgress(3, 4000);

    expect(progress.value).toBe(2700);
    expect(progress.percent).toBe(100);
    expect(progress.canLevelUp).toBe(true);
  });

  it('Edge: the exported paths name the identity values', () => {
    expect([XP_PATH, LEVEL_PATH]).toEqual(['identity.xp', 'identity.level']);
  });
});

describe('the experience gain parser', () => {
  it('Edge: empty, unreadable, zero and negative gains are ignored', () => {
    expect(parseXpGain('')).toBeUndefined();
    expect(parseXpGain('   ')).toBeUndefined();
    expect(parseXpGain('abc')).toBeUndefined();
    expect(parseXpGain('NaN')).toBeUndefined();
    expect(parseXpGain('Infinity')).toBeUndefined();
    expect(parseXpGain('1e999')).toBeUndefined();
    expect(parseXpGain('0')).toBeUndefined();
    expect(parseXpGain('-0')).toBeUndefined();
    expect(parseXpGain('-25')).toBeUndefined();
  });

  it('Happy path: a positive gain reads as a finite number, surrounding blanks trimmed', () => {
    expect(parseXpGain(' 250 ')).toBe(250);
    expect(parseXpGain('2.5')).toBe(2.5);
  });
});

describe('the hero experience bar', () => {
  it('Happy path: the bar spans the current floor to the next threshold', async () => {
    await openXp({ level: 3, xp: 1400 });

    const bar = xpBar();
    expect(bar).toHaveAttribute('aria-valuemin', '900');
    expect(bar).toHaveAttribute('aria-valuemax', '2700');
    expect(bar).toHaveAttribute('aria-valuenow', '1400');
    expect(screen.queryByTestId('sheet-xp-panel')).not.toBeInTheDocument();
  });

  it('Edge: level 20 reads as a full bar at the cap', async () => {
    await openXp({ level: 20, xp: 355000 });

    const bar = xpBar();
    expect(bar).toHaveAttribute('aria-valuemin', '355000');
    expect(bar).toHaveAttribute('aria-valuemax', '355000');
    expect(bar).toHaveAttribute('aria-valuenow', '355000');
  });

  it('Edge: experience below the floor reads as an empty bar at the floor', async () => {
    await openXp({ level: 5, xp: 100 });

    const bar = xpBar();
    expect(bar).toHaveAttribute('aria-valuemin', '6500');
    expect(bar).toHaveAttribute('aria-valuemax', '14000');
    expect(bar).toHaveAttribute('aria-valuenow', '6500');
  });

  it('Happy path: the bar opens and closes the experience panel', async () => {
    const { user } = await openXp({ level: 3, xp: 1400 });

    expect(xpToggle()).toHaveAttribute('aria-expanded', 'false');
    const panel = await openPanel(user);
    expect(panel).toHaveTextContent(messageWith('sheet.play.xpTotal', { count: 1400 }));
    expect(gainInput()).toBeInTheDocument();
    expect(xpToggle()).toHaveAttribute('aria-expanded', 'true');

    await user.click(xpToggle());
    expect(screen.queryByTestId('sheet-xp-panel')).not.toBeInTheDocument();
    expect(xpToggle()).toHaveAttribute('aria-expanded', 'false');
  });

  it('Happy path: a gain adds to the pending experience total', async () => {
    const { fake, user } = await openXp({ level: 3, xp: 1400 });
    await openPanel(user);

    await user.type(gainInput(), '200{Enter}');

    expect(screen.getByTestId('sheet-xp-panel')).toHaveTextContent(
      messageWith('sheet.play.xpTotal', { count: 1600 }),
    );
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { xp: 1600 } } }));
  });

  it('Edge: an empty or negative gain writes nothing', async () => {
    const { fake, user } = await openXp({ level: 3, xp: 1400 });
    await openPanel(user);

    await user.click(screen.getByRole('button', { name: message('sheet.play.xpAdd') }));
    await user.type(gainInput(), '-50');
    await user.click(screen.getByRole('button', { name: message('sheet.play.xpAdd') }));

    expect(screen.getByTestId('sheet-xp-panel')).toHaveTextContent(
      messageWith('sheet.play.xpTotal', { count: 1400 }),
    );
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: the level-up offer waits until the gain reaches the next threshold', async () => {
    const { user } = await openXp({ level: 3, xp: 2600 });
    await openPanel(user);

    expect(
      screen.queryByRole('button', {
        name: messageWith('sheet.play.levelUp', { count: 4 }),
      }),
    ).not.toBeInTheDocument();

    await user.type(gainInput(), '100{Enter}');

    expect(
      screen.getByRole('button', { name: messageWith('sheet.play.levelUp', { count: 4 }) }),
    ).toBeInTheDocument();
  });

  it('Happy path: reaching the threshold and accepting the level up bumps the pending level', async () => {
    const { fake, user } = await openXp({ level: 3, xp: 2700 });
    await openPanel(user);

    await user.click(
      screen.getByRole('button', { name: messageWith('sheet.play.levelUp', { count: 4 }) }),
    );

    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 4 }),
    );
    expect(
      screen.queryByRole('button', { name: messageWith('sheet.play.levelUp', { count: 4 }) }),
    ).not.toBeInTheDocument();
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 4 } } }));
  });

  it('Edge: level 20 never offers a level up', async () => {
    const { user } = await openXp({ level: 20, xp: 355000 });
    await openPanel(user);

    expect(screen.queryByRole('button', { name: /^Level up to/ })).not.toBeInTheDocument();
  });
});
