import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { Toast } from '../ui/toast';

describe('notice primitive (UIP-23…UIP-26)', () => {
  it('Happy path: polite live region with message', () => {
    render(<Toast message="Saved" />);
    expect(screen.getByRole('status')).toHaveTextContent('Saved');
  });
  it('Happy path: the dismiss control is a named icon button that calls onDismiss', async () => {
    const onDismiss = vi.fn();
    render(<Toast message="Saved" onDismiss={onDismiss} />);
    const dismiss = screen.getByRole('button', { name: message('ui.dismiss') });
    expect(dismiss).toHaveAttribute('type', 'button');
    expect(dismiss).toHaveTextContent('');
    expect(dismiss.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    await userEvent.setup().click(dismiss);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
  it('Edge: without onDismiss there is no dismiss control', () => {
    render(<Toast message="Saved" />);
    expect(screen.queryByRole('button')).toBeNull();
  });
  it('Edge: empty message renders nothing', () => {
    const { container } = render(<Toast message="   " />);
    expect(container.querySelector('[role="status"]')).toBeNull();
  });
});
