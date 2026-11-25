import { message } from '../l10n/strings';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  DerivationChain,
  STRENGTH_MAX,
  STRENGTH_MIN,
  deriveFromStrength,
  signed,
} from '../landing/derivation_chain';

describe('deriveFromStrength', () => {
  it.each([
    ['Edge: the lowest Strength gives a negative modifier', STRENGTH_MIN, -1, 1, 120],
    ['Edge: an odd score rounds the modifier down', 17, 3, 5, 255],
    ['Edge: the highest Strength', STRENGTH_MAX, 5, 7, 300],
    ['Edge: a score of 9 floors toward negative', 9, -1, 1, 135],
    ['Happy path: the starting Strength', 16, 3, 5, 240],
  ])('%s', (_name, strength, modifier, attack, carry) => {
    expect(deriveFromStrength(strength)).toEqual({ modifier, attack, carry });
  });
});

describe('signed', () => {
  it.each([
    ['Edge: zero is shown as positive', 0, '+0'],
    ['Edge: negatives keep their sign', -2, '-2'],
    ['Happy path: positives gain a plus', 3, '+3'],
  ])('%s', (_name, value, text) => {
    expect(signed(value)).toBe(text);
  });
});

describe('DerivationChain', () => {
  const raise = () => fireEvent.click(screen.getByRole('button', { name: 'Raise Strength' }));
  const lower = () => fireEvent.click(screen.getByRole('button', { name: 'Lower Strength' }));

  it('Happy path: raising Strength recalculates every derived value', () => {
    render(<DerivationChain />);
    raise();
    raise();
    expect(screen.getByTestId('landing-chain-strength')).toHaveTextContent('18');
    expect(screen.getByTestId('landing.chainModifier-value')).toHaveTextContent('+4');
    expect(screen.getByTestId('landing.chainAttack-value')).toHaveTextContent('+6');
    expect(screen.getByTestId('landing.chainCarry-value')).toHaveTextContent('270 lb');
  });

  it('Edge: Strength stops at the maximum and disables raising', () => {
    render(<DerivationChain />);
    for (let i = 0; i < 10; i++) raise();
    expect(screen.getByTestId('landing-chain-strength')).toHaveTextContent(String(STRENGTH_MAX));
    expect(screen.getByRole('button', { name: 'Raise Strength' })).toBeDisabled();
  });

  it('Edge: Strength stops at the minimum and disables lowering', () => {
    render(<DerivationChain />);
    for (let i = 0; i < 12; i++) lower();
    expect(screen.getByTestId('landing-chain-strength')).toHaveTextContent(String(STRENGTH_MIN));
    expect(screen.getByRole('button', { name: 'Lower Strength' })).toBeDisabled();
    expect(screen.getByTestId('landing.chainModifier-value')).toHaveTextContent('-1');
  });

  it('Happy path: the chain is a group named by its own level-2 heading', () => {
    render(<DerivationChain />);
    const heading = screen.getByRole('heading', { level: 2, name: message('landing.chainTitle') });
    expect(screen.getByRole('group', { name: message('landing.chainTitle') })).toContainElement(
      heading,
    );
  });

  it('Edge: the chain title and the steps title say different things', () => {
    expect(message('landing.chainTitle')).not.toBe(message('landing.howTitle'));
    expect(message('landing.chainTitle').trim()).not.toBe('');
    expect(message('landing.howTitle').trim()).not.toBe('');
  });
});
