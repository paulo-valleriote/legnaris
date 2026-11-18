import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EmptyState, ErrorState, LoadingState } from '../ui/state';
import { messages } from '../l10n/strings';

describe('state primitives', () => {
  it('Happy path: an empty state is one muted line with the optional action', () => {
    const { container } = render(
      <EmptyState message="No sheets yet." action={<button type="button">Create one</button>} />,
    );
    expect(screen.getByText('No sheets yet.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create one' })).toBeInTheDocument();
    expect(container.querySelector('.lg-state--empty')).not.toBeNull();
    expect(container.querySelector('.lg-state--empty')!.querySelector('h2')).toBeNull();
  });

  it('Happy path: an empty state can lead with a title', () => {
    render(<EmptyState title="Nothing here" message="No sheets yet." />);
    expect(screen.getByText('Nothing here')).toBeInTheDocument();
    expect(screen.getByText('No sheets yet.')).toBeInTheDocument();
  });

  it('Happy path: the error state is an alert with the default retry wording', async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    render(<ErrorState message="That failed." onRetry={retry} />);
    expect(screen.getByRole('alert')).toHaveTextContent('That failed.');
    await user.click(screen.getByRole('button', { name: messages['common.retry'] }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it('Edge: an error state without a retry renders no button', () => {
    render(<ErrorState message="That failed." />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('Happy path: a custom retry label wins over the shared wording', () => {
    render(<ErrorState message="That failed." onRetry={() => {}} retryLabel="Reload sheet" />);
    expect(screen.getByRole('button', { name: 'Reload sheet' })).toBeInTheDocument();
  });

  it('Happy path: the loading state is busy, muted and defaults to the shared label', () => {
    const { container } = render(<LoadingState />);
    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-busy', 'true');
    expect(status).toHaveTextContent(messages['common.loading']);
    expect(container.querySelector('.lg-spinner')).not.toBeNull();
  });

  it('Happy path: a caller-supplied loading label replaces the default', () => {
    render(<LoadingState label="Loading your campaigns" />);
    expect(screen.getByRole('status')).toHaveTextContent('Loading your campaigns');
  });

  it('Happy path: every state accepts an id and an extra class', () => {
    const { container } = render(
      <>
        <EmptyState id="empty-region" className="lg-test__empty" message="No sheets yet." />
        <ErrorState id="error-region" className="lg-test__error" message="That failed." />
        <LoadingState id="loading-region" className="lg-test__loading" />
      </>,
    );

    expect(document.getElementById('empty-region')).toHaveClass(
      'lg-state--empty',
      'lg-test__empty',
    );
    expect(document.getElementById('error-region')).toHaveClass(
      'lg-state--error',
      'lg-test__error',
    );
    expect(document.getElementById('loading-region')).toHaveClass(
      'lg-state--loading',
      'lg-test__loading',
    );
    expect(container.querySelectorAll('[id$="-region"]')).toHaveLength(3);
  });
});
