import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { DECLARED_ROUTES, PRIVACY_PATH } from '../app/routes';
import { renderApp } from './render';

describe('privacy route (LGN-234)', () => {
  it('Happy path: the privacy route is declared public with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === PRIVACY_PATH);

    expect(route).toMatchObject({ audience: 'public', deepLink: true, notFound: false });
  });

  it('Happy path: a visitor without a session reads the policy on a direct navigation', () => {
    renderApp(PRIVACY_PATH, { session: 'unauthenticated' });

    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
    expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument();
  });

  it('Edge: a session-only path still sends a visitor without a session to sign-in', () => {
    renderApp('/account', { session: 'unauthenticated' });

    expect(
      screen.queryByRole('heading', { level: 1, name: 'Your account' }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { level: 1, name: /Privacy policy/ }),
    ).not.toBeInTheDocument();
  });
});
