import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { UpdatePromptView } from '../pwa/update_prompt';

describe('update prompt view (FND-09)', () => {
  it('Edge: renders nothing when there is no update and no offline state', () => {
    render(
      <UpdatePromptView
        needRefresh={false}
        offlineReady={false}
        onUpdate={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.queryByTestId('update-prompt')).not.toBeInTheDocument();
  });

  it('Happy path: shows a non-blocking notice when an update is pending', () => {
    render(
      <UpdatePromptView needRefresh offlineReady={false} onUpdate={vi.fn()} onDismiss={vi.fn()} />,
    );

    expect(screen.getByTestId('update-prompt')).toBeInTheDocument();
    expect(screen.getByText('A new version is available.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument();
  });

  it('Happy path: accepting the update calls the update handler', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    render(
      <UpdatePromptView needRefresh offlineReady={false} onUpdate={onUpdate} onDismiss={vi.fn()} />,
    );

    await user.click(screen.getByRole('button', { name: 'Reload' }));

    expect(onUpdate).toHaveBeenCalledTimes(1);
  });

  it('Edge: the offline-ready state offers no reload action', () => {
    render(
      <UpdatePromptView needRefresh={false} offlineReady onUpdate={vi.fn()} onDismiss={vi.fn()} />,
    );

    expect(screen.getByText('Ready to work offline.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Reload' })).not.toBeInTheDocument();
  });
});
