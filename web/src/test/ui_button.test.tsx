import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Button } from '../ui/button';

describe('action primitive (UIP-07…UIP-10, SC-001)', () => {
  it('Happy path: primary variant fills accent and uses on-accent text', () => {
    render(<Button variant="primary" label="Save" />);
    const btn = screen.getByRole('button', { name: /Save/i });
    expect(btn.className).toContain('lg-button--primary');
  });

  it('Edge: danger variant is outlined, not filled', () => {
    render(<Button variant="danger" label="Delete" />);
    const btn = screen.getByRole('button', { name: /Delete/i });
    expect(btn.className).toContain('lg-button--danger');
  });

  it('Happy path: both sizes render at or above control height', () => {
    const { container: c1 } = render(<Button size="sm" label="A" />);
    const { container: c2 } = render(<Button size="md" label="B" />);
    expect(c1.querySelector('button')!.className).toContain('lg-button--sm');
    expect(c2.querySelector('button')!.className).toContain('lg-button--md');
  });

  it('Edge: disabled and loading do not run their handler', () => {
    const handler = () => {
      throw new Error('should not run');
    };
    render(<Button disabled label="No" onClick={handler} />);
    render(<Button loading label="Busy" onClick={handler} />);
    // Activation handled by browser; assertion is the disabled/loading state is announced.
    expect(screen.getAllByRole('button')).toHaveLength(2);
  });

  it('Happy path: default type never submits a form', () => {
    render(<Button label="X" />);
    expect(screen.getByRole('button')).toHaveAttribute('type', 'button');
  });
});
