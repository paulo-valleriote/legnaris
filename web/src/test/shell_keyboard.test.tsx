import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderApp } from './render';

describe('shell keyboard access', () => {
  it('Happy path: the first control is reachable with the keyboard alone (FND-04)', async () => {
    const user = userEvent.setup();
    renderApp('/');

    await user.tab();

    expect(screen.getByRole('link', { name: 'Catalog' })).toHaveFocus();
  });

  it('Edge: tabbing past the last control does not trap focus in the shell (FND-04)', async () => {
    const user = userEvent.setup();
    renderApp('/');
    const link = screen.getByRole('link', { name: 'Catalog' });

    await user.tab();
    expect(link).toHaveFocus();

    await user.tab();
    expect(link).not.toHaveFocus();
  });

  it('Edge: the not-found screen keeps its link reachable by keyboard (FND-04)', async () => {
    const user = userEvent.setup();
    renderApp('/unknown');

    await user.tab();
    await user.tab();

    expect(screen.getByRole('link', { name: /Return to the workspace/i })).toHaveFocus();
  });
});
