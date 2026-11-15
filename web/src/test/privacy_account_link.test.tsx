import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const PLAN = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
  llm_consent_at: null,
  llm_consent_version: null,
};

function accountClient(plan: ReturnType<typeof successOutcome> | ReturnType<typeof errorOutcome>) {
  const fake = createFakeTransport(successOutcome([]), plan);
  return createApiClient({ baseUrl: 'http://api.test', transport: fake.transport });
}

describe('account privacy link (LGN-234)', () => {
  it('Happy path: the account screen links to the privacy policy route', () => {
    renderApp('/account', { client: accountClient(successOutcome(PLAN)) });

    const link = screen.getByRole('link', { name: message('account.privacyLink') });
    expect(link).toHaveAttribute('href', '/privacy');
  });

  it('Edge: the link stays on the account screen when the plan fails to load', async () => {
    renderApp('/account', {
      client: accountClient(errorOutcome('INTERNAL', 'The server had a problem.', 500)),
    });

    expect(await screen.findByTestId('account-failure')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: message('account.privacyLink') })).toHaveAttribute(
      'href',
      '/privacy',
    );
  });
});
