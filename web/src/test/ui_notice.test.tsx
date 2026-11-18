import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Notice } from '../ui/notice';

describe('notice strip primitive', () => {
  it.each([
    ['info', 'lg-notice--info'],
    ['warning', 'lg-notice--warning'],
    ['danger', 'lg-notice--danger'],
    ['success', 'lg-notice--success'],
  ] as const)('Happy path: %s renders its tone class', (tone, expected) => {
    const { container } = render(<Notice tone={tone}>Heads up</Notice>);
    expect(container.querySelector(`.${expected}`)).not.toBeNull();
    expect(screen.getByText('Heads up')).toBeInTheDocument();
  });

  it('Happy path: the polite notice is a status region', () => {
    render(
      <Notice tone="warning" live="polite">
        Payment failed
      </Notice>,
    );
    expect(screen.getByRole('status')).toHaveTextContent('Payment failed');
  });

  it('Edge: by default the notice is not a live region', () => {
    render(<Notice tone="info">Standing condition</Notice>);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('Happy path: the optional action renders at the trailing edge', async () => {
    const user = userEvent.setup();
    const action = vi.fn();
    render(
      <Notice
        tone="warning"
        action={
          <button type="button" onClick={action}>
            Fix it
          </button>
        }
      >
        Payment failed
      </Notice>,
    );
    await user.click(screen.getByRole('button', { name: 'Fix it' }));
    expect(action).toHaveBeenCalledTimes(1);
  });

  it('Happy path: the stage surface and its id and extra class are applied', () => {
    const { container } = render(
      <Notice tone="info" surface="stage" id="payment-notice" className="lg-test__notice">
        Heads up
      </Notice>,
    );
    const notice = document.getElementById('payment-notice');

    expect(notice).toHaveClass('lg-notice--info', 'lg-notice--stage', 'lg-test__notice');
    expect(container.querySelector('.lg-notice')).toBe(notice);
  });

  it('Edge: the app surface is the default and carries no stage class', () => {
    const { container } = render(<Notice tone="danger">Payment failed</Notice>);
    const notice = container.querySelector('.lg-notice') as HTMLElement;

    expect(notice).toHaveClass('lg-notice--danger');
    expect(notice).not.toHaveClass('lg-notice--stage');
  });
});
