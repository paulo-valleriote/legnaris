import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { DiceResult } from '../dice/errors';
import { refuse, succeed } from '../dice/errors';
import type { RollPart, RollResult } from '../dice/roll';
import { message, messageWith } from '../l10n/strings';
import { DicePanel } from '../sheet/dice_panel';
import type { RollEntry } from '../sheet/dice_history';

afterEach(cleanup);

function entry(
  id: number,
  label: string,
  total: number,
  parts: readonly RollPart[] = [],
): RollEntry {
  return { id, label, outcome: succeed({ total, parts }) };
}

const rolled = entry(1, 'Initiative', 14, [
  { sign: 1, label: '1d20', value: 11, rolls: [11] },
  { sign: 1, label: 'attrs.dex', value: 3 },
]);

function labels(): (string | null)[] {
  return [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__total-label')].map(
    (label) => label.textContent,
  );
}

describe('DicePanel', () => {
  it('Happy path: shows the total and every die and number that made it', () => {
    render(<DicePanel results={[rolled]} onDismiss={() => {}} />);

    expect(screen.getByText('Initiative')).toBeTruthy();
    expect(screen.getByText('14')).toBeTruthy();
    expect(screen.getByText('1d20')).toBeTruthy();
    expect(screen.getByText(messageWith('dice.rolls', { rolls: '11' }))).toBeTruthy();
    expect(screen.getByText('attrs.dex')).toBeTruthy();
  });

  it('Happy path: a minimum that applied is shown with the result', () => {
    const floored: RollEntry = {
      id: 1,
      label: 'Initiative',
      outcome: succeed({
        total: 5,
        minimum: 5,
        parts: [{ sign: 1, label: '1d6', value: 2, rolls: [2] }],
      }),
    };
    render(<DicePanel results={[floored]} onDismiss={() => {}} />);

    expect(document.querySelector('.lg-dice-panel__minimum')?.textContent).toBe(
      messageWith('dice.minimum', { minimum: 5 }),
    );
  });

  it('Edge: a minimum that did not apply draws no line', () => {
    render(<DicePanel results={[rolled]} onDismiss={() => {}} />);

    expect(document.querySelector('.lg-dice-panel__minimum')).toBeNull();
  });

  it('Edge: an empty history draws no panel', () => {
    render(<DicePanel results={[]} onDismiss={() => {}} />);

    expect(document.querySelector('.lg-dice-panel')).toBeNull();
  });

  it('Happy path: three results are listed newest first with their own totals and parts', () => {
    const newest = entry(3, 'Damage', 9, [{ sign: 1, label: '2d6', value: 9, rolls: [4, 5] }]);
    const middle = entry(2, 'Attack', 18, [{ sign: 1, label: '1d20', value: 18, rolls: [18] }]);
    const oldest = entry(1, 'Initiative', 7, [{ sign: 1, label: '1d20', value: 7, rolls: [7] }]);

    render(<DicePanel results={[newest, middle, oldest]} onDismiss={() => {}} />);

    expect(labels()).toEqual(['Damage', 'Attack', 'Initiative']);
    const entries = document.querySelectorAll<HTMLElement>('.lg-dice-panel__entry');
    expect(entries).toHaveLength(3);
    expect(entries[0].querySelector('.lg-dice-panel__total-value')?.textContent).toBe('9');
    expect(entries[0].querySelector('.lg-dice-panel__parts')?.textContent).toContain('2d6');
    expect(entries[1].querySelector('.lg-dice-panel__total-value')?.textContent).toBe('18');
    expect(entries[2].querySelector('.lg-dice-panel__total-value')?.textContent).toBe('7');
  });

  it('Happy path: the result is announced through one polite live region', () => {
    render(<DicePanel results={[rolled]} onDismiss={() => {}} />);

    const statuses = screen.getAllByRole('status');
    expect(statuses).toHaveLength(1);
    expect(statuses[0].getAttribute('aria-live')).toBe('polite');
    expect(statuses[0].textContent).toBe(
      messageWith('dice.rolled', { label: 'Initiative', total: 14 }),
    );
  });

  it('Edge: only the newest result is announced, never the older ones', () => {
    const newest = entry(3, 'Damage', 9);
    const middle = entry(2, 'Attack', 18);
    const oldest = entry(1, 'Initiative', 7);

    render(<DicePanel results={[newest, middle, oldest]} onDismiss={() => {}} />);

    const statuses = screen.getAllByRole('status');
    expect(statuses).toHaveLength(1);
    expect(statuses[0].textContent).toBe(messageWith('dice.rolled', { label: 'Damage', total: 9 }));
    expect(statuses[0].textContent).not.toContain('Attack');
    expect(statuses[0].textContent).not.toContain('Initiative');
  });

  it('Edge: the live region holds the sentence alone, never the total or the dismiss control', () => {
    render(<DicePanel results={[rolled]} onDismiss={() => {}} />);

    const status = screen.getByRole('status');
    const dismiss = screen.getByRole('button', { name: message('common.close') });

    expect(status).toHaveClass('lg-dice-panel__sr');
    expect(status.querySelector('button')).toBeNull();
    expect(status.contains(dismiss)).toBe(false);
    expect(document.querySelector('.lg-dice-panel__total')?.contains(status)).toBe(false);
  });

  it('Edge: a missing or non-numeric value announces its explicit message and shows no total', () => {
    const missing: DiceResult<RollResult> = refuse('no_value', '"attrs.dex" has no number to read');
    render(
      <DicePanel
        results={[{ id: 1, label: 'Initiative', outcome: missing }]}
        onDismiss={() => {}}
      />,
    );

    expect(screen.getByRole('status').textContent).toBe(message('dice.error.noValue'));
    expect(document.querySelector('.lg-dice-panel__error')?.textContent).toBe(
      message('dice.error.noValue'),
    );
    expect(document.querySelector('.lg-dice-panel__total-value')).toBeNull();
  });

  it('Edge: an older failed roll keeps its own label and message in the history', () => {
    const newest = entry(2, 'Attack', 11);
    const failed: DiceResult<RollResult> = refuse('no_dice', 'dice are off');
    render(
      <DicePanel
        results={[newest, { id: 1, label: 'Initiative', outcome: failed }]}
        onDismiss={() => {}}
      />,
    );

    expect(labels()).toEqual(['Attack', 'Initiative']);
    const entries = document.querySelectorAll<HTMLElement>('.lg-dice-panel__entry');
    expect(entries[1].textContent).toContain(message('dice.error.noDice'));
    expect(entries[1].querySelector('.lg-dice-panel__total-value')).toBeNull();
  });

  it('Edge: the panel is not a modal dialog and does not block the sheet', () => {
    render(<DicePanel results={[rolled]} onDismiss={() => {}} />);

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.querySelector('[aria-modal="true"]')).toBeNull();
  });

  it('Happy path: the dismiss button removes the panel through its callback', async () => {
    const onDismiss = vi.fn();
    render(<DicePanel results={[rolled]} onDismiss={onDismiss} />);

    await userEvent.click(screen.getByRole('button', { name: message('common.close') }));

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it('Edge: Escape dismisses the panel from anywhere inside it', () => {
    const onDismiss = vi.fn();
    render(<DicePanel results={[rolled]} onDismiss={onDismiss} />);

    fireEvent.keyDown(screen.getByRole('status'), { key: 'Escape' });

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it('Edge: a key other than Escape does not dismiss the panel', () => {
    const onDismiss = vi.fn();
    render(<DicePanel results={[rolled]} onDismiss={onDismiss} />);

    fireEvent.keyDown(screen.getByRole('status'), { key: 'Enter' });

    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('Happy path: an advantaged roll lists both dice, marks the dropped one and names the kept', () => {
    const advantaged: RollEntry = {
      id: 1,
      label: 'Initiative',
      mode: 'advantage',
      outcome: succeed({
        total: 17,
        parts: [{ sign: 1, label: '1d20', value: 17, rolls: [4, 17], discarded: [0] }],
      }),
    };
    render(<DicePanel results={[advantaged]} onDismiss={() => {}} />);

    const dice = [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__die')];
    expect(dice.map((die) => die.textContent)).toEqual(['4', '17']);
    expect(dice[0]).toHaveClass('lg-dice-panel__die--dropped');
    expect(dice[1]).toHaveClass('lg-dice-panel__die--kept');
    expect(
      screen.getByText(messageWith('dice.keptRolls', { rolls: '4, 17', kept: '17' })),
    ).toBeTruthy();
    expect(screen.getByText(message('dice.mode.advantage'))).toBeTruthy();
  });

  it('Edge: a disadvantaged roll marks the lower die, not the higher one', () => {
    const disadvantaged: RollEntry = {
      id: 1,
      label: 'Initiative',
      mode: 'disadvantage',
      outcome: succeed({
        total: 4,
        parts: [{ sign: 1, label: '1d20', value: 4, rolls: [4, 17], discarded: [1] }],
      }),
    };
    render(<DicePanel results={[disadvantaged]} onDismiss={() => {}} />);

    const dice = [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__die')];
    expect(dice[0]).toHaveClass('lg-dice-panel__die--kept');
    expect(dice[1]).toHaveClass('lg-dice-panel__die--dropped');
    expect(screen.getByText(message('dice.mode.disadvantage'))).toBeTruthy();
  });

  it('Edge: both dice equal keep the first and mark only the second as dropped', () => {
    const tied: RollEntry = {
      id: 1,
      label: 'Initiative',
      mode: 'advantage',
      outcome: succeed({
        total: 9,
        parts: [{ sign: 1, label: '1d20', value: 9, rolls: [9, 9], discarded: [1] }],
      }),
    };
    render(<DicePanel results={[tied]} onDismiss={() => {}} />);

    const dice = [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__die')];
    expect(dice.map((die) => die.textContent)).toEqual(['9', '9']);
    expect(dice[0]).toHaveClass('lg-dice-panel__die--kept');
    expect(dice[1]).toHaveClass('lg-dice-panel__die--dropped');
  });

  it('Edge: a normal roll names no mode and draws no mode mark', () => {
    render(<DicePanel results={[rolled]} onDismiss={() => {}} />);

    expect(document.querySelector('.lg-dice-panel__mode')).toBeNull();
    expect(document.querySelector('.lg-dice-panel__die')).toBeNull();
    expect(screen.queryByText(message('dice.mode.advantage'))).toBeNull();
  });

  it('Happy path: an advantaged result is announced with its mode', () => {
    const advantaged: RollEntry = {
      id: 2,
      label: 'Initiative',
      mode: 'advantage',
      outcome: succeed({ total: 17, parts: [] }),
    };
    render(<DicePanel results={[advantaged, rolled]} onDismiss={() => {}} />);

    expect(screen.getByRole('status').textContent).toBe(
      messageWith('dice.rolledAdvantage', { label: 'Initiative', total: 17 }),
    );
    expect(screen.getByRole('status').textContent).not.toContain('11');
  });

  it('Edge: a negative term is shown with its minus sign, not dropped', () => {
    const withMinus = entry(1, 'Hit', 2, [
      { sign: 1, label: '1d20', value: 5, rolls: [5] },
      { sign: -1, label: 'attrs.str', value: 3 },
    ]);
    render(<DicePanel results={[withMinus]} onDismiss={() => {}} />);

    expect(screen.getByText('-')).toBeTruthy();
    expect(screen.getByText('3')).toBeTruthy();
  });
});
