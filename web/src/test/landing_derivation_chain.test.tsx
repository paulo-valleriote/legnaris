import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  DerivationChain,
  MIGHT_MAX,
  MIGHT_MIN,
  deriveFromMight,
  signed,
} from '../landing/derivation_chain';

describe('deriveFromMight', () => {
  it.each([
    ['Edge: the lowest Might gives a negative modifier', MIGHT_MIN, -1, 1, 120],
    ['Edge: an odd score rounds the modifier down', 17, 3, 5, 255],
    ['Edge: the highest Might', MIGHT_MAX, 5, 7, 300],
    ['Edge: a score of 9 floors toward negative', 9, -1, 1, 135],
    ['Happy path: the starting Might', 16, 3, 5, 240],
  ])('%s', (_name, might, modifier, attack, carry) => {
    expect(deriveFromMight(might)).toEqual({ modifier, attack, carry });
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
  const raise = () => fireEvent.click(screen.getByRole('button', { name: 'Raise Might' }));
  const lower = () => fireEvent.click(screen.getByRole('button', { name: 'Lower Might' }));

  it('Happy path: raising Might recalculates every derived value', () => {
    render(<DerivationChain />);
    raise();
    raise();
    expect(screen.getByTestId('landing-chain-might')).toHaveTextContent('18');
    expect(screen.getByTestId('landing.chainModifier-value')).toHaveTextContent('+4');
    expect(screen.getByTestId('landing.chainAttack-value')).toHaveTextContent('+6');
    expect(screen.getByTestId('landing.chainCarry-value')).toHaveTextContent('270 lb');
  });

  it('Edge: Might stops at the maximum and disables raising', () => {
    render(<DerivationChain />);
    for (let i = 0; i < 10; i++) raise();
    expect(screen.getByTestId('landing-chain-might')).toHaveTextContent(String(MIGHT_MAX));
    expect(screen.getByRole('button', { name: 'Raise Might' })).toBeDisabled();
  });

  it('Edge: Might stops at the minimum and disables lowering', () => {
    render(<DerivationChain />);
    for (let i = 0; i < 12; i++) lower();
    expect(screen.getByTestId('landing-chain-might')).toHaveTextContent(String(MIGHT_MIN));
    expect(screen.getByRole('button', { name: 'Lower Might' })).toBeDisabled();
    expect(screen.getByTestId('landing.chainModifier-value')).toHaveTextContent('-1');
  });
});
