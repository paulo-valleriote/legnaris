import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { Sheet } from '../api/models/sheet';
import { message, messageWith } from '../l10n/strings';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { conditionIsOn, conditionLabelParts } from '../sheet/preset/compose/condition_label';
import { Conditions } from '../sheet/preset/compose/conditions';
import {
  hitPointsPercent,
  parseHitPointsAmount,
  parseHitPointsValue,
} from '../sheet/preset/compose/hit_points';
import { HitPointsTrack } from '../sheet/preset/compose/hit_points_track';
import { Pips } from '../sheet/preset/compose/pips';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import type {
  PresetConditionsPiece,
  PresetConfig,
  PresetHitPointsPiece,
  PresetPiece,
} from '../sheet/preset/compose/types';
import { PendingChangesCard } from '../sheet/preset/pending_card';
import { PresetPlayView } from '../sheet/preset/play_view';
import { presetConfig, presetSheet, presetTemplateDetail, renderPreset } from './preset_fixtures';

const BLINDED = 'status.conditions.blinded';
const PRONE = 'status.conditions.prone';
const BLINDED_LABEL = 'Blinded: cannot see and automatically fails sight checks';
const BLINDED_RULE = 'cannot see and automatically fails sight checks';

function hitPoints(overrides: Partial<PresetHitPointsPiece> = {}): PresetHitPointsPiece {
  return {
    kind: 'hitPoints',
    key: 'vitals',
    current: 'vitals.hit',
    max: 'vitals.hitMax',
    temp: 'vitals.hitTemp',
    ...overrides,
  };
}

function conditions(overrides: Partial<PresetConditionsPiece> = {}): PresetConditionsPiece {
  return {
    kind: 'conditions',
    key: 'status',
    conditions: [{ path: BLINDED }, { path: PRONE }],
    ...overrides,
  };
}

function vitalsSheet(vitals: Record<string, unknown>): Sheet {
  return presetSheet({
    data: {
      identity: { name: 'Nova', subtitle: 'Scout' },
      vitals: { armor: 14, hit: 12, hitMax: 20, hitTemp: 0, empty: '', ...vitals },
    },
  });
}

function conditionsSheet(extra: Record<string, unknown>): Sheet {
  return presetSheet({
    data: {
      identity: { name: 'Nova', subtitle: 'Scout' },
      status: { conditions: { blinded: 0, prone: 0 } },
      exhaustion: { level: 0, penalty: 0 },
      ...extra,
    },
  });
}

function undoRegion(): HTMLElement {
  const region = document.querySelector<HTMLElement>('.lg-compose-hp__undo');
  if (region === null) {
    throw new Error('undo region missing');
  }
  return region;
}

function amountInput(): HTMLElement {
  return screen.getByRole('textbox', { name: message('sheet.play.amount') });
}

function pendingCard(): HTMLElement | null {
  return screen.queryByTestId('sheet-pending-card');
}

function renderHitPointsWithPendingCard(hit: number, hitTemp = 0) {
  return renderPreset(
    <>
      <HitPointsTrack piece={hitPoints()} />
      <PendingChangesCard />
    </>,
    { sheet: vitalsSheet({ hit, hitTemp, hitMax: 24 }) },
  );
}

describe('the hit point track piece', () => {
  it('Happy path: draws the current, max and temp leaves with their original labels', () => {
    renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 10, hitTemp: 3 }),
    });

    expect(screen.getByRole('button', { name: 'Hit Points' })).toHaveTextContent('10');
    expect(screen.getByRole('button', { name: 'Hit Points Max' })).toHaveTextContent('20');
    expect(screen.getByRole('button', { name: 'Temporary Hit Points' })).toHaveTextContent('3');
  });

  it('Happy path: the meter carries the current and maximum', () => {
    renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 10 }),
    });

    const meter = screen.getByRole('meter', { name: message('sheet.play.hitPoints') });
    expect(meter).toHaveAttribute('aria-valuemin', '0');
    expect(meter).toHaveAttribute('aria-valuemax', '20');
    expect(meter).toHaveAttribute('aria-valuenow', '10');
  });

  it('Happy path: damage commits the current and lets the temp absorb it first', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12, hitTemp: 3 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 10, hitTemp: 0 } });
  });

  it('Happy path: heal commits the current', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 10 }),
    });

    await user.type(amountInput(), '4');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 14 } });
  });

  it('Happy path: undo after damage restores the current and the absorbed temp', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12, hitTemp: 3 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 10, hitTemp: 0 } });

    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(store.getPendingDelta()).toEqual({});
    expect(screen.getByRole('button', { name: 'Hit Points' })).toHaveTextContent('12');
    expect(screen.getByRole('button', { name: 'Temporary Hit Points' })).toHaveTextContent('3');
    expect(
      screen.queryByRole('button', { name: message('sheet.play.hpUndo') }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: undo after heal restores the previous current', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 10 }),
    });

    await user.type(amountInput(), '4');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));
    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 14 } });
    expect(undoRegion()).toHaveTextContent(messageWith('sheet.play.healed', { count: 4 }));

    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(store.getPendingDelta()).toEqual({});
    expect(screen.getByRole('button', { name: 'Hit Points' })).toHaveTextContent('10');
  });

  it('Edge: a second apply replaces the undo target with the state before it', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    await user.type(amountInput(), '3');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 4 } });
    expect(undoRegion()).toHaveTextContent(messageWith('sheet.play.tookDamage', { count: 3 }));

    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 7 } });
    expect(screen.getByRole('button', { name: 'Hit Points' })).toHaveTextContent('7');
  });

  it('Happy path: undo after damage clears the pending hit point change', async () => {
    const user = userEvent.setup();
    const { store } = renderHitPointsWithPendingCard(24);

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    expect(pendingCard()).toHaveTextContent('Hit Points 24 → 19');

    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(pendingCard()).not.toBeInTheDocument();
    expect([...store.getState().editedPaths]).toEqual([]);
    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: undo after damage that spent temporary hit points clears every change', async () => {
    const user = userEvent.setup();
    const { store } = renderHitPointsWithPendingCard(24, 5);

    await user.type(amountInput(), '8');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    expect(pendingCard()).toHaveTextContent('Hit Points 24 → 21');
    expect(pendingCard()).toHaveTextContent('Temporary Hit Points 5 → 0');

    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(pendingCard()).not.toBeInTheDocument();
    expect([...store.getState().editedPaths]).toEqual([]);
    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: undo restores an earlier unsaved hit point edit and keeps it pending', async () => {
    const user = userEvent.setup();
    const { store } = renderHitPointsWithPendingCard(24);
    await user.click(screen.getByRole('button', { name: 'Hit Points' }));
    const hitPointsInput = screen.getByRole('textbox', { name: 'Hit Points' });
    await user.clear(hitPointsInput);
    await user.type(hitPointsInput, '20{Enter}');

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    expect(pendingCard()).toHaveTextContent('Hit Points 24 → 15');

    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(pendingCard()).toHaveTextContent('Hit Points 24 → 20');
    expect(pendingCard()).not.toHaveTextContent('Hit Points 24 → 15');
    expect([...store.getState().editedPaths]).toEqual(['vitals.hit']);
    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 20 } });
  });

  it('Edge: undo keeps an unrelated pending change in the card', async () => {
    const user = userEvent.setup();
    const { store } = renderHitPointsWithPendingCard(24);
    act(() => {
      store.setPath('identity.name', 'Brenna');
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
    await user.click(screen.getByRole('button', { name: message('sheet.play.hpUndo') }));

    expect(pendingCard()).toHaveTextContent('Name Nova → Brenna');
    expect(pendingCard()).not.toHaveTextContent('Hit Points');
    expect([...store.getState().editedPaths]).toEqual(['identity.name']);
    expect(store.getPendingDelta()).toEqual({ identity: { name: 'Brenna' } });
  });

  it('Edge: the undo affordance clears after eight seconds', () => {
    vi.useFakeTimers();
    try {
      renderPreset(<HitPointsTrack piece={hitPoints()} />, {
        sheet: vitalsSheet({ hit: 12 }),
      });

      fireEvent.change(amountInput(), { target: { value: '5' } });
      fireEvent.click(screen.getByRole('button', { name: message('sheet.play.damage') }));
      expect(
        screen.getByRole('button', { name: message('sheet.play.hpUndo') }),
      ).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(7999);
      });
      expect(
        screen.getByRole('button', { name: message('sheet.play.hpUndo') }),
      ).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(1);
      });
      expect(
        screen.queryByRole('button', { name: message('sheet.play.hpUndo') }),
      ).not.toBeInTheDocument();
      expect(undoRegion()).toBeEmptyDOMElement();
    } finally {
      vi.useRealTimers();
    }
  });

  it.each([
    { label: 'zero', text: '0' },
    { label: 'empty', text: '' },
  ])('Edge: a $label amount offers no undo', async ({ text }) => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    if (text !== '') {
      await user.type(amountInput(), text);
    }
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({});
    expect(
      screen.queryByRole('button', { name: message('sheet.play.hpUndo') }),
    ).not.toBeInTheDocument();
    expect(undoRegion()).toBeEmptyDOMElement();
  });

  it('Happy path: the status line names the change and the undo button is named Undo', async () => {
    const user = userEvent.setup();
    renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    const status = undoRegion();
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(status).toHaveTextContent(messageWith('sheet.play.tookDamage', { count: 5 }));
    expect(within(status).getByRole('button', { name: 'Undo' })).toBeInTheDocument();
  });

  it('Edge: damage larger than temp plus current floors the current at zero', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 4, hitTemp: 1 }),
    });

    await user.type(amountInput(), '20');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 0, hitTemp: 0 } });
  });

  it('Edge: heal above the maximum clamps to the maximum', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 10 }),
    });

    await user.type(amountInput(), '15');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 20 } });
  });

  it('Edge: a heal never lowers the current when the maximum is zero', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 10, hitMax: 0 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a heal never lowers a current above the maximum', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 40, hitMax: 30 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a heal at the maximum commits nothing', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 20 }),
    });

    await user.type(amountInput(), '5');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a zero amount is ignored', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    await user.type(amountInput(), '0');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: an empty amount is ignored', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Error: a non-numeric amount is rejected', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    await user.type(amountInput(), 'abc');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({});
    expect(screen.getByRole('button', { name: 'Hit Points' })).toHaveTextContent('12');
  });

  it('Happy path: the amount plus Enter damages from the keyboard', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    await user.type(amountInput(), '2{Enter}');

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 10 } });
  });

  it('Edge: a missing temp binding draws no temp control and damages the current alone', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<HitPointsTrack piece={hitPoints({ temp: undefined })} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    expect(screen.queryByRole('button', { name: 'Temporary Hit Points' })).not.toBeInTheDocument();

    await user.type(amountInput(), '3');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(store.getPendingDelta()).toEqual({ vitals: { hit: 9 } });
  });

  it('Edge: a derived maximum is read-only', () => {
    renderPreset(<HitPointsTrack piece={hitPoints({ max: 'vitals.derivedMax' })} />, {
      sheet: vitalsSheet({}),
    });

    expect(screen.queryByRole('button', { name: 'Derived Max' })).not.toBeInTheDocument();
    expect(screen.getByText('Derived Max')).toBeInTheDocument();
  });

  it('Edge: a non-positive maximum draws an empty meter', () => {
    renderPreset(<HitPointsTrack piece={hitPoints({ max: 0 })} />, {
      sheet: vitalsSheet({ hit: 12 }),
    });

    const meter = screen.getByRole('meter', { name: message('sheet.play.hitPoints') });
    expect(meter).toHaveAttribute('aria-valuenow', '0');
  });

  it('Edge: a non-finite stored current reads as zero', () => {
    renderPreset(<HitPointsTrack piece={hitPoints()} />, {
      sheet: vitalsSheet({ hit: 'lots' }),
    });

    expect(screen.getByRole('button', { name: 'Hit Points' })).toHaveTextContent('0');
  });
});

describe('the pips piece', () => {
  it('Edge: a zero maximum draws no label and no pips', () => {
    renderPreset(<Pips piece={{ kind: 'pips', key: 'empty', count: 'charges.used', max: 0 }} />);

    expect(screen.queryByText('Charges Used')).not.toBeInTheDocument();
    expect(document.querySelector('.lg-compose-pips')).toBeNull();
  });

  it('Happy path: a filled pip offers to clear and an empty one to set', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(
      <Pips piece={{ kind: 'pips', key: 'charges', count: 'charges.used', max: 4 }} />,
    );

    expect(screen.getByRole('button', { name: 'Clear Charges Used to 0' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Set Charges Used to 2' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Clear Charges Used to 0' }));

    expect(store.getPendingDelta()).toEqual({ charges: { used: 0 } });
  });
});

describe('the conditions piece', () => {
  it('Happy path: each flag draws its original label and the rule text as its description', () => {
    renderPreset(<Conditions piece={conditions()} />, { sheet: conditionsSheet({}) });

    const blinded = screen.getByRole('button', { name: BLINDED_LABEL });
    expect(blinded).toHaveTextContent('Blinded');
    expect(blinded).toHaveAccessibleDescription(BLINDED_RULE);
    expect(screen.getByRole('button', { name: 'Prone' })).toHaveTextContent('Prone');
  });

  it('Happy path: a stored flag reads pressed', () => {
    renderPreset(<Conditions piece={conditions()} />, {
      sheet: conditionsSheet({ status: { conditions: { blinded: 1, prone: 0 } } }),
    });

    expect(screen.getByRole('button', { name: BLINDED_LABEL })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: 'Prone' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('Happy path: a toggle commits 1 and toggles back to 0', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Conditions piece={conditions()} />, {
      sheet: conditionsSheet({}),
    });

    await user.click(screen.getByRole('button', { name: BLINDED_LABEL }));
    expect(store.getPendingDelta()).toEqual({ status: { conditions: { blinded: 1 } } });

    await user.click(screen.getByRole('button', { name: BLINDED_LABEL }));
    expect(store.getPendingDelta()).toEqual({});
  });

  it('Happy path: toggling a stored condition off commits 0', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Conditions piece={conditions()} />, {
      sheet: conditionsSheet({ status: { conditions: { blinded: 1, prone: 0 } } }),
    });

    await user.click(screen.getByRole('button', { name: BLINDED_LABEL }));

    expect(store.getPendingDelta()).toEqual({ status: { conditions: { blinded: 0 } } });
  });

  it('Happy path: a toggle works from the keyboard alone', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Conditions piece={conditions()} />, {
      sheet: conditionsSheet({}),
    });

    screen.getByRole('button', { name: BLINDED_LABEL }).focus();
    await user.keyboard('{Enter}');

    expect(store.getPendingDelta()).toEqual({ status: { conditions: { blinded: 1 } } });
  });

  it('Edge: a non-numeric stored flag reads off', () => {
    renderPreset(<Conditions piece={conditions()} />, {
      sheet: conditionsSheet({ status: { conditions: { blinded: 'yes', prone: null } } }),
    });

    expect(screen.getByRole('button', { name: BLINDED_LABEL })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('Edge: a label without rule text draws no description', () => {
    renderPreset(<Conditions piece={conditions()} />, { sheet: conditionsSheet({}) });

    expect(screen.getByRole('button', { name: 'Prone' })).toHaveAccessibleDescription('');
  });

  it('Edge: no bound flag and no exhaustion draws nothing', () => {
    renderPreset(<Conditions piece={conditions({ conditions: [] })} />, {
      sheet: conditionsSheet({}),
    });

    expect(screen.queryByTestId('compose-conditions')).not.toBeInTheDocument();
  });
});

describe('the conditions piece exhaustion track', () => {
  const exhaustion = { level: 'exhaustion.level', max: 6, penalty: 'exhaustion.penalty' };

  it('Happy path: the level draws pips out of the maximum and the bound penalty', () => {
    renderPreset(<Conditions piece={conditions({ exhaustion })} />, {
      sheet: conditionsSheet({ exhaustion: { level: 2, penalty: -4 } }),
    });

    const pips = screen.getAllByRole('button', { name: /Exhaustion level/ });
    expect(pips).toHaveLength(6);
    expect(pips.filter((pip) => pip.getAttribute('aria-pressed') === 'true')).toHaveLength(2);
    expect(screen.getByText('Exhaustion penalty')).toBeInTheDocument();
    expect(screen.getByText('-4')).toBeInTheDocument();
  });

  it('Happy path: a pip commits the new level', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Conditions piece={conditions({ exhaustion })} />, {
      sheet: conditionsSheet({}),
    });

    await user.click(screen.getAllByRole('button', { name: /Exhaustion level/ })[3]);

    expect(store.getPendingDelta()).toEqual({ exhaustion: { level: 4 } });
  });

  it('Edge: level zero fills no pip and the maximum fills every pip', () => {
    const { unmount } = renderPreset(<Conditions piece={conditions({ exhaustion })} />, {
      sheet: conditionsSheet({}),
    });
    const atZero = screen.getAllByRole('button', { name: /Exhaustion level/ });
    expect(atZero.every((pip) => pip.getAttribute('aria-pressed') === 'false')).toBe(true);
    unmount();

    renderPreset(<Conditions piece={conditions({ exhaustion })} />, {
      sheet: conditionsSheet({ exhaustion: { level: 6, penalty: -12 } }),
    });
    const atMax = screen.getAllByRole('button', { name: /Exhaustion level/ });
    expect(atMax.every((pip) => pip.getAttribute('aria-pressed') === 'true')).toBe(true);
  });

  it('Edge: an omitted penalty binding draws no penalty line', () => {
    renderPreset(
      <Conditions piece={conditions({ exhaustion: { level: 'exhaustion.level', max: 6 } })} />,
      { sheet: conditionsSheet({}) },
    );

    expect(screen.queryByText('Exhaustion penalty')).not.toBeInTheDocument();
  });

  it('Edge: an exhaustion track with no bound flag still draws', () => {
    renderPreset(<Conditions piece={conditions({ conditions: [], exhaustion })} />, {
      sheet: conditionsSheet({}),
    });

    expect(screen.getByRole('group', { name: /Exhaustion level/ })).toBeInTheDocument();
  });
});

describe('the vitals pure helpers', () => {
  it.each([
    { label: 'Blinded: cannot see', expected: { name: 'Blinded', rule: 'cannot see' } },
    { label: 'Prone', expected: { name: 'Prone' } },
    { label: 'Spaced  :  rule text ', expected: { name: 'Spaced', rule: 'rule text' } },
    { label: ': leading colon', expected: { name: ': leading colon' } },
    { label: 'Trailing:', expected: { name: 'Trailing:' } },
    { label: '', expected: { name: '' } },
  ])('Edge: splits the label "$label" into a chip name and rule text', ({ label, expected }) => {
    expect(conditionLabelParts(label)).toEqual(expected);
  });

  it.each([
    { value: 0, expected: false },
    { value: 1, expected: true },
    { value: 2, expected: true },
    { value: -1, expected: true },
    { value: Number.NaN, expected: false },
    { value: '1', expected: false },
    { value: null, expected: false },
    { value: undefined, expected: false },
  ])('Edge: the stored flag $value reads $expected', ({ value, expected }) => {
    expect(conditionIsOn(value)).toBe(expected);
  });

  it.each([
    { text: '5', expected: 5 },
    { text: ' 5 ', expected: 5 },
    { text: '0', expected: undefined },
    { text: '-2', expected: undefined },
    { text: '2.5', expected: undefined },
    { text: 'abc', expected: undefined },
    { text: '', expected: undefined },
    { text: 'Infinity', expected: undefined },
  ])('Error: an amount of "$text" parses to $expected', ({ text, expected }) => {
    expect(parseHitPointsAmount(text)).toBe(expected);
  });

  it.each([
    { text: '0', expected: 0 },
    { text: '12', expected: 12 },
    { text: '-1', expected: undefined },
    { text: '1.5', expected: undefined },
    { text: '', expected: undefined },
  ])('Edge: a stored value of "$text" parses to $expected', ({ text, expected }) => {
    expect(parseHitPointsValue(text)).toBe(expected);
  });

  it.each([
    { current: 10, max: 20, expected: 50 },
    { current: 30, max: 20, expected: 100 },
    { current: -5, max: 20, expected: 0 },
    { current: 5, max: 0, expected: 0 },
    { current: 5, max: Number.NaN, expected: 0 },
  ])('Edge: $current of $max is $expected percent', ({ current, max, expected }) => {
    expect(hitPointsPercent(current, max)).toBe(expected);
  });
});

describe('the vitals pieces in the config contract', () => {
  function config(pieces: readonly PresetPiece[]): PresetConfig {
    return presetConfig({ pieces });
  }

  const template = presetTemplateDetail();

  it('Happy path: a config with the vitals pieces has no path errors', () => {
    const valid = config([
      hitPoints(),
      conditions({
        exhaustion: { level: 'exhaustion.level', max: 6, penalty: 'exhaustion.penalty' },
      }),
    ]);

    expect(configPathErrors(valid, template)).toEqual([]);
  });

  it.each([
    {
      name: 'an unknown hit point current',
      piece: hitPoints({ current: 'vitals.ghost' }),
      expected: ['vitals.ghost'],
    },
    {
      name: 'an unknown hit point max',
      piece: hitPoints({ max: 'vitals.ghost' }),
      expected: ['vitals.ghost'],
    },
    {
      name: 'an unknown hit point temp',
      piece: hitPoints({ temp: 'vitals.ghost' }),
      expected: ['vitals.ghost'],
    },
    {
      name: 'an unknown condition flag',
      piece: conditions({ conditions: [{ path: 'status.conditions.ghost' }] }),
      expected: ['status.conditions.ghost'],
    },
    {
      name: 'an unknown exhaustion level',
      piece: conditions({ exhaustion: { level: 'exhaustion.ghost', max: 6 } }),
      expected: ['exhaustion.ghost'],
    },
    {
      name: 'an unknown exhaustion penalty',
      piece: conditions({
        exhaustion: { level: 'exhaustion.level', max: 6, penalty: 'exhaustion.ghost' },
      }),
      expected: ['exhaustion.ghost'],
    },
    {
      name: 'an unknown exhaustion max leaf',
      piece: conditions({
        exhaustion: { level: 'exhaustion.level', max: 'exhaustion.ghost' },
      }),
      expected: ['exhaustion.ghost'],
    },
  ])('Error: $name is reported', ({ piece, expected }) => {
    expect(configPathErrors(config([piece]), template)).toEqual(expected);
  });

  it('Edge: a numeric exhaustion maximum binds no path', () => {
    const numeric = config([
      conditions({ conditions: [], exhaustion: { level: 'exhaustion.level', max: 6 } }),
    ]);

    expect(configPathErrors(numeric, template)).toEqual([]);
  });

  it('Happy path: the adapter registers both pieces in their listed order', () => {
    const preset = presetFromConfig(config([hitPoints(), conditions({ exhaustion: undefined })]));

    renderPreset(
      <>
        <preset.Header />
        <PresetPlayView preset={preset} />
      </>,
      { sheet: vitalsSheet({ hitTemp: 3 }) },
    );

    expect(
      screen.getByRole('meter', { name: message('sheet.play.hitPoints') }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: BLINDED_LABEL })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Temporary Hit Points' })).toHaveTextContent('3');
  });
});
