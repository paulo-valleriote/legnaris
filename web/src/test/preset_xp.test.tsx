import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import { XpBar } from '../sheet/preset/compose/xp_bar';
import {
  parseXpGain,
  parseXpTotal,
  XP_THRESHOLDS,
  xpProgress,
} from '../sheet/preset/compose/xp_progress';
import type { PresetXpBarPiece } from '../sheet/preset/compose/types';
import { DiceProvider } from '../sheet/dice_row';
import { PresetProvider } from '../sheet/preset/preset_context';
import { fifthCharacterSheet, fifthCharacterTemplate } from './fifth_fixtures';
import { createPresetRuntime } from './preset_fixtures';

const PIECE: PresetXpBarPiece = {
  kind: 'xpBar',
  key: 'xp',
  level: 'identity.level',
  xp: 'status.xp',
};

function sheetWithXp(xp: number) {
  const sheet = fifthCharacterSheet();
  return {
    ...sheet,
    data: {
      ...(sheet.data as Record<string, unknown>),
      status: {
        ...((sheet.data as Record<string, unknown>).status as Record<string, unknown>),
        xp,
      },
    },
  };
}

function renderXp(options: { readonly xp?: number; readonly editing?: boolean } = {}) {
  const runtime = createPresetRuntime(fifthCharacterTemplate(), sheetWithXp(options.xp ?? 0));
  return {
    ...render(
      <PresetProvider {...runtime.preset} editing={options.editing ?? true}>
        <DiceProvider value={runtime.dice}>
          <XpBar piece={PIECE} />
        </DiceProvider>
      </PresetProvider>,
    ),
    store: runtime.store,
  };
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
    const progress = xpProgress(5, 14000);

    expect(progress.nextLevel).toBe(6);
    expect(progress.nextThreshold).toBe(14000);
    expect(progress.value).toBe(14000);
    expect(progress.percent).toBe(100);
    expect(progress.canLevelUp).toBe(true);
  });

  it('Edge: level 20 fills the bar and never offers a next level', () => {
    const atCap = xpProgress(20, 355000);

    expect(atCap.nextLevel).toBeNull();
    expect(atCap.nextThreshold).toBeNull();
    expect(atCap.canLevelUp).toBe(false);
    expect(atCap.percent).toBe(100);
  });

  it('Edge: gain and total parsers reject empty, negative and non-numeric text', () => {
    expect(parseXpGain('')).toBeUndefined();
    expect(parseXpGain('0')).toBeUndefined();
    expect(parseXpGain('-5')).toBeUndefined();
    expect(parseXpGain('lots')).toBeUndefined();
    expect(parseXpGain('25')).toBe(25);
    expect(parseXpTotal('')).toBeUndefined();
    expect(parseXpTotal('-1')).toBe(0);
    expect(parseXpTotal('900')).toBe(900);
  });
});

describe('the experience bar on a 5E sheet', () => {
  it('Happy path: the bar reads the stored level and experience', () => {
    renderXp({ xp: 6500 });

    const bar = screen.getByRole('progressbar', { name: message('sheet.play.experience') });
    expect(bar).toHaveAttribute('aria-valuemin', String(XP_THRESHOLDS[4]));
    expect(bar).toHaveAttribute('aria-valuemax', String(XP_THRESHOLDS[5]));
    expect(bar).toHaveAttribute('aria-valuenow', '6500');
    expect(screen.getByTestId('sheet-xp')).toHaveTextContent('6,500 / 14,000');
  });

  it('Happy path: Gain XP adds to the stored total', async () => {
    const user = userEvent.setup();
    const { store } = renderXp();

    await user.click(screen.getByTestId('sheet-xp-trigger'));
    await user.type(screen.getByRole('spinbutton', { name: message('sheet.play.xpGain') }), '300');
    await user.click(screen.getByRole('button', { name: message('sheet.play.xpAdd') }));

    expect((store.getState().data as { status: { xp: number } }).status.xp).toBe(300);
  });

  it('Happy path: a threshold reached offers Level up and raises the level', async () => {
    const user = userEvent.setup();
    const { store } = renderXp({ xp: 14000 });

    await user.click(screen.getByTestId('sheet-xp-trigger'));
    await user.click(
      screen.getByRole('button', { name: messageWith('sheet.play.levelUp', { count: 6 }) }),
    );

    expect((store.getState().data as { identity: { level: number } }).identity.level).toBe(6);
  });

  it('Edge: on a locked sheet the total is read-only, while Gain XP and Level up stay live', async () => {
    const user = userEvent.setup();
    const { store } = renderXp({ xp: 14000, editing: false });

    await user.click(screen.getByTestId('sheet-xp-trigger'));

    expect(screen.queryByRole('textbox', { name: message('sheet.play.setXp') })).toBeNull();
    await user.click(
      screen.getByRole('button', { name: messageWith('sheet.play.levelUp', { count: 6 }) }),
    );
    expect((store.getState().data as { identity: { level: number } }).identity.level).toBe(6);
  });
});
