import { describe, expect, it } from 'vitest';
import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { valueAtPath } from '../api/models/sheet';
import { message, messageWith } from '../l10n/strings';
import { HitPointsTrack } from '../sheet/preset/compose/hit_points_track';
import { Pips } from '../sheet/preset/compose/pips';
import type { PresetHitPointsPiece } from '../sheet/preset/compose/types';
import { fifthCharacterSheet, fifthCharacterTemplate } from './fifth_fixtures';
import { renderPreset } from './preset_fixtures';

const piece: PresetHitPointsPiece = {
  kind: 'hitPoints',
  key: 'hitPoints',
  current: 'combat.hp.current',
  max: 24,
  temp: 'combat.hp.temp',
  downNotice: 'preset.hitPoints.downSaves',
};

function renderTrack(extras: Partial<PresetHitPointsPiece> = {}) {
  return renderPreset(<HitPointsTrack piece={{ ...piece, ...extras }} />, {
    template: fifthCharacterTemplate(),
    sheet: fifthCharacterSheet(),
  });
}

function chip(key: 'sheet.play.damageAmount' | 'sheet.play.healAmount', count: number) {
  return screen.getByRole('button', { name: messageWith(key, { count }) });
}

const down = () => document.querySelector('.lg-compose-hp--down');

describe('the hit point quick changes', () => {
  it('Edge: damage spends temporary points first and floors the current value at zero', async () => {
    const user = userEvent.setup();
    const { store } = renderTrack();
    act(() => {
      store.setPath('combat.hp.current', 3);
      store.setPath('combat.hp.temp', 2);
    });

    await user.click(chip('sheet.play.damageAmount', 5));
    await user.click(chip('sheet.play.damageAmount', 5));

    expect(valueAtPath(store.getState().data, 'combat.hp.temp')).toBe(0);
    expect(valueAtPath(store.getState().data, 'combat.hp.current')).toBe(0);
  });

  it('Edge: healing never passes the maximum', async () => {
    const user = userEvent.setup();
    const { store } = renderTrack();
    act(() => store.setPath('combat.hp.current', 22));

    await user.click(chip('sheet.play.healAmount', 5));

    expect(valueAtPath(store.getState().data, 'combat.hp.current')).toBe(24);
  });

  it('Edge: a quick change leaves a typed amount alone', async () => {
    const user = userEvent.setup();
    renderTrack();
    const amount = screen.getByRole('textbox', { name: message('sheet.play.amount') });

    await user.type(amount, '7');
    await user.click(chip('sheet.play.damageAmount', 1));

    expect(amount).toHaveValue('7');
  });

  it('Happy path: a quick change can be undone', async () => {
    const user = userEvent.setup();
    const { store } = renderTrack();
    act(() => store.setPath('combat.hp.current', 10));

    await user.click(chip('sheet.play.damageAmount', 5));
    await user.click(screen.getByRole('button', { name: /undo/i }));

    expect(valueAtPath(store.getState().data, 'combat.hp.current')).toBe(10);
  });
});

describe('the zero hit points state', () => {
  it('Edge: a zero maximum is not treated as down', () => {
    const { store } = renderTrack({ max: 0 });

    act(() => store.setPath('combat.hp.current', 0));

    expect(down()).toBeNull();
  });

  it('Edge: a track without a notice never draws the state', () => {
    const { store } = renderTrack({ downNotice: undefined });

    act(() => store.setPath('combat.hp.current', 0));

    expect(down()).toBeNull();
  });

  it('Happy path: reaching zero announces it and healing clears it', async () => {
    const user = userEvent.setup();
    const { store } = renderTrack();
    act(() => store.setPath('combat.hp.current', 1));

    await user.click(chip('sheet.play.damageAmount', 1));

    expect(down()).not.toBeNull();
    expect(screen.getByText(message('preset.hitPoints.downSaves'))).toHaveAttribute(
      'role',
      'status',
    );

    await user.click(chip('sheet.play.healAmount', 1));

    expect(down()).toBeNull();
    expect(screen.queryByText(message('preset.hitPoints.downSaves'))).not.toBeInTheDocument();
  });
});

describe('the decorated pip track', () => {
  it('Edge: the short label is visual only, the accessible names keep the field label', () => {
    renderPreset(
      <Pips
        piece={{
          kind: 'pips',
          key: 'successes',
          count: 'status.deathSaves.successes',
          max: 3,
          heading: 'preset.deathSaves.title',
          label: 'preset.deathSaves.successes',
          critical: true,
        }}
      />,
      { template: fifthCharacterTemplate(), sheet: fifthCharacterSheet() },
    );

    const track = document.querySelector<HTMLElement>('.lg-compose-pips--critical');
    expect(track).not.toBeNull();
    const view = within(track as HTMLElement);
    expect(view.getByRole('heading', { name: message('preset.deathSaves.title') })).toBeVisible();
    expect(view.getByText(message('preset.deathSaves.successes'))).toBeVisible();
    expect(view.getByRole('group', { name: /Death save successes, \d of 3/ })).toBeInTheDocument();
  });

  it('Happy path: the visible count follows the filled pips', async () => {
    const user = userEvent.setup();
    renderPreset(
      <Pips
        piece={{ kind: 'pips', key: 'failures', count: 'status.deathSaves.failures', max: 3 }}
      />,
      { template: fifthCharacterTemplate(), sheet: fifthCharacterSheet() },
    );

    await user.click(screen.getAllByRole('button')[1] as HTMLElement);

    expect(document.querySelector('.lg-compose-pips__count')).toHaveTextContent('2/3');
  });
});
