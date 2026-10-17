import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { renderApp } from './render';

describe('shell keyboard access', () => {
  it('Happy path: the first control is still the catalog link (FND-04)', async () => {
    const user = userEvent.setup();
    renderApp('/');

    await user.tab();

    expect(screen.getByRole('link', { name: message('nav.catalog') })).toHaveFocus();
  });

  it('Happy path: the theme control is reachable in the navigation tab order (UIF-18, UIF-27)', async () => {
    const user = userEvent.setup();
    renderApp('/');
    const control = screen.getByRole('button', { name: message('theme.toggleLabel') });

    let reached = false;
    for (let step = 0; step < 6 && !reached; step += 1) {
      await user.tab();
      reached = document.activeElement === control;
    }

    expect(reached).toBe(true);
  });

  it('Edge: tabbing past the last control does not trap focus in the shell (FND-04)', async () => {
    const user = userEvent.setup();
    renderApp('/');
    const link = screen.getByRole('link', { name: message('nav.catalog') });

    await user.tab();
    expect(link).toHaveFocus();

    await user.tab();
    expect(link).not.toHaveFocus();
  });

  it('Edge: the not-found screen keeps its link reachable by keyboard (FND-04)', async () => {
    const user = userEvent.setup();
    renderApp('/unknown');

    // The navigation offers the catalog link, the own-sheet link and the footer's
    // sign-out and theme controls (WAS-19, UIF-27) ahead of the content link.
    for (let step = 0; step < 5; step += 1) {
      await user.tab();
    }

    expect(screen.getByRole('link', { name: /Return to the workspace/i })).toHaveFocus();
  });
});
