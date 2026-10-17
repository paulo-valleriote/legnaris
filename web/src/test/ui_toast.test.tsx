import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Toast } from '../ui/toast';

describe('notice primitive (UIP-23…UIP-26)', () => {
  it('Happy path: polite live region with message', () => {
    render(<Toast message="Saved" />);
    expect(screen.getByRole('status')).toHaveTextContent('Saved');
  });
  it('Edge: empty message renders nothing', () => {
    const { container } = render(<Toast message="   " />);
    expect(container.querySelector('[role="status"]')).toBeNull();
  });
});
