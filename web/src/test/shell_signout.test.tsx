import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { AuthApi } from '../api/auth_api';
import type { AuthSession } from '../auth/auth_session';
import { createAuthSession } from '../auth/auth_session';
import { message } from '../l10n/strings';
import { renderApp } from './render';

function unusedAuthApi(): AuthApi {
  const unused = () => Promise.reject(new Error('auth operation is not scripted for this suite'));
  return {
    register: unused,
    login: unused,
    refresh: unused,
    logout: () => Promise.resolve(),
    verifyChallenge: unused,
    resendChallenge: unused,
  };
}

function authenticatedSession(): AuthSession {
  return createAuthSession({
    authApi: unusedAuthApi(),
    initialState: 'authenticated',
    initialAccessToken: 'test-access-token',
  });
}

describe('navigation sign-out', () => {
  it('Happy path: the action reports progress until the session ends', async () => {
    const user = userEvent.setup();
    let release: () => void = () => undefined;
    const logout = () =>
      new Promise<void>((resolve) => {
        release = resolve;
      });
    renderApp('/', { session: { ...authenticatedSession(), logout } });

    await user.click(screen.getByRole('button', { name: message('nav.logout') }));

    const busy = screen.getByRole('button', { name: message('common.signingOut') });
    expect(busy).toHaveAttribute('aria-busy', 'true');
    expect(busy).toBeDisabled();

    release();

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('nav.logout') })).not.toHaveAttribute(
        'aria-busy',
      ),
    );
  });

  it('Error: a failed sign-out says so inline instead of failing silently', async () => {
    const user = userEvent.setup();
    renderApp('/', {
      session: { ...authenticatedSession(), logout: () => Promise.reject(new Error('offline')) },
    });

    await user.click(screen.getByRole('button', { name: message('nav.logout') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('common.signOutFailed'));

    const control = screen.getByRole('button', { name: message('nav.logout') });
    expect(control).not.toHaveAttribute('aria-busy');
    expect(control).toHaveAccessibleDescription(message('common.signOutFailed'));
  });
});
