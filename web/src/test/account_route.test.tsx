import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { DECLARED_ROUTES } from '../app/routes';

const PLAN = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
  llm_consent_at: null,
  llm_consent_version: null,
};

describe('account route (LGN-195)', () => {
  it('Happy path: the account route is declared as a session route with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === '/account');

    expect(route).toMatchObject({ audience: 'session', deepLink: true, notFound: false });
  });

  it('Happy path: the left navigation links to the account and a click opens it', async () => {
    const fake = createFakeTransport(successOutcome([]), successOutcome(PLAN));
    const user = userEvent.setup();
    renderApp('/', {
      client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
    });

    const link = screen.getByRole('link', { name: message('nav.account') });
    expect(link).toHaveAttribute('href', '/account');

    await user.click(link);

    expect(await screen.findByRole('heading', { name: message('account.title') })).toBeVisible();
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
  });

  it('Edge: a direct navigation to the account route renders the screen inside the shell', async () => {
    const fake = createFakeTransport(successOutcome(PLAN));
    renderApp('/account', {
      client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
    });

    expect(await screen.findByTestId('account-plan')).toHaveTextContent(
      message('account.planFree'),
    );
    expect(screen.getByRole('main')).toContainElement(screen.getByTestId('account-plan'));
  });
});
