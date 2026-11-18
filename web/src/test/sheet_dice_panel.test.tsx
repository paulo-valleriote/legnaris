import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { DiceResult } from '../dice/errors';
import { refuse, succeed } from '../dice/errors';
import type { RollResult } from '../dice/roll';
import { message, messageWith } from '../l10n/strings';
import { DicePanel } from '../sheet/dice_panel';

afterEach(cleanup);

const rolled: DiceResult<RollResult> = succeed({
  total: 14,
  parts: [
    { sign: 1, label: '1d20', value: 11, rolls: [11] },
    { sign: 1, label: 'attrs.dex', value: 3 },
  ],
});

describe('DicePanel', () => {
  it('Happy path: shows the total and every die and number that made it', () => {
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={() => {}} />);

    expect(screen.getByText('Initiative')).toBeTruthy();
    expect(screen.getByText('14')).toBeTruthy();
    expect(screen.getByText('1d20')).toBeTruthy();
    expect(screen.getByText('attrs.dex')).toBeTruthy();
  });

  it('Happy path: the result is announced through one polite live region', () => {
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={() => {}} />);

    const statuses = screen.getAllByRole('status');
    expect(statuses).toHaveLength(1);
    expect(statuses[0].getAttribute('aria-live')).toBe('polite');
    expect(statuses[0].textContent).toBe(
      messageWith('dice.rolled', { label: 'Initiative', total: 14 }),
    );
  });

  it('Edge: the live region holds the sentence alone, never the total or the dismiss control', () => {
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={() => {}} />);

    const status = screen.getByRole('status');
    const dismiss = screen.getByRole('button', { name: message('common.close') });

    expect(status).toHaveClass('lg-dice-panel__sr');
    expect(status.querySelector('button')).toBeNull();
    expect(status.contains(dismiss)).toBe(false);
    expect(document.querySelector('.lg-dice-panel__total')?.contains(status)).toBe(false);
  });

  it('Edge: a missing or non-numeric value announces its explicit message and shows no total', () => {
    const missing: DiceResult<RollResult> = refuse('no_value', '"attrs.dex" has no number to read');
    render(<DicePanel label="Initiative" outcome={missing} onDismiss={() => {}} />);

    expect(screen.getByRole('status').textContent).toBe(message('dice.error.noValue'));
    expect(document.querySelector('.lg-dice-panel__error')?.textContent).toBe(
      message('dice.error.noValue'),
    );
    expect(document.querySelector('.lg-dice-panel__total-value')).toBeNull();
  });

  it('Edge: the panel is not a modal dialog and does not block the sheet', () => {
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={() => {}} />);

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.querySelector('[aria-modal="true"]')).toBeNull();
  });

  it('Happy path: the dismiss button removes the panel through its callback', async () => {
    const onDismiss = vi.fn();
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={onDismiss} />);

    await userEvent.click(screen.getByRole('button', { name: message('common.close') }));

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it('Edge: Escape dismisses the panel from anywhere inside it', () => {
    const onDismiss = vi.fn();
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={onDismiss} />);

    fireEvent.keyDown(screen.getByRole('status'), { key: 'Escape' });

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it('Edge: a key other than Escape does not dismiss the panel', () => {
    const onDismiss = vi.fn();
    render(<DicePanel label="Initiative" outcome={rolled} onDismiss={onDismiss} />);

    fireEvent.keyDown(screen.getByRole('status'), { key: 'Enter' });

    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('Edge: a negative term is shown with its minus sign, not dropped', () => {
    const withMinus: DiceResult<RollResult> = succeed({
      total: 2,
      parts: [
        { sign: 1, label: '1d20', value: 5, rolls: [5] },
        { sign: -1, label: 'attrs.str', value: 3 },
      ],
    });
    render(<DicePanel label="Hit" outcome={withMinus} onDismiss={() => {}} />);

    expect(screen.getByText('-')).toBeTruthy();
    expect(screen.getByText('3')).toBeTruthy();
  });
});
