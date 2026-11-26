import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentType } from 'react';
import { message } from '../l10n/strings';
import { MomentsSlot } from '../landing/moments_slot';

function Loaded() {
  return <p>moments ready</p>;
}

const loaded = { default: Loaded as ComponentType };

describe('landing moments slot', () => {
  it('Happy path: the loaded moments replace the skeleton', async () => {
    render(<MomentsSlot load={() => Promise.resolve(loaded)} />);

    expect(await screen.findByText('moments ready')).toBeInTheDocument();
    expect(screen.queryByTestId('landing-moments-loading')).not.toBeInTheDocument();
  });

  it('Edge: while loading, three card shells hold the space and the status names the wait', () => {
    render(<MomentsSlot load={() => new Promise(() => {})} />);

    const skeleton = screen.getByTestId('landing-moments-loading');
    expect(skeleton).toHaveAttribute('role', 'status');
    expect(skeleton).toHaveTextContent(message('landing.momentsLoading'));
    expect(skeleton.querySelectorAll('.lg-landing__moment-shell')).toHaveLength(3);
  });

  it('Error: a failed chunk shows the recovery message instead of crashing the page', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    render(<MomentsSlot load={() => Promise.reject(new Error('chunk failed'))} />);

    expect(await screen.findByTestId('landing-moments-failed')).toHaveTextContent(
      message('landing.momentsFailed'),
    );
    vi.restoreAllMocks();
  });

  it('Happy path: retrying after a failure loads the moments', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const user = userEvent.setup();
    const load = vi
      .fn<() => Promise<typeof loaded>>()
      .mockRejectedValueOnce(new Error('chunk failed'))
      .mockResolvedValue(loaded);
    render(<MomentsSlot load={load} />);

    await user.click(await screen.findByRole('button', { name: message('landing.momentsRetry') }));

    expect(await screen.findByText('moments ready')).toBeInTheDocument();
    expect(load).toHaveBeenCalledTimes(2);
    vi.restoreAllMocks();
  });

  it('Edge: a second failure keeps the retry available', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const user = userEvent.setup();
    const load = vi.fn(() => Promise.reject(new Error('chunk failed')));
    render(<MomentsSlot load={load} />);

    await user.click(await screen.findByRole('button', { name: message('landing.momentsRetry') }));

    expect(
      await screen.findByRole('button', { name: message('landing.momentsRetry') }),
    ).toBeInTheDocument();
    expect(load).toHaveBeenCalledTimes(2);
    vi.restoreAllMocks();
  });
});
