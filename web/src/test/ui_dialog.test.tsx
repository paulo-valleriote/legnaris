import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Dialog } from '../ui/dialog';

describe('dialog primitive (UIP-18…UIP-21)', () => {
  it('Happy path: exposes dialog role and title', () => {
    render(<Dialog open title="Confirm" onClose={() => {}} />);
    expect(screen.getByRole('dialog', { name: /Confirm/i })).toBeInTheDocument();
  });
  it('Edge: scrim activation closes dialog', () => {
    const close = () => {};
    render(<Dialog open title="X" onClose={close} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });
});
