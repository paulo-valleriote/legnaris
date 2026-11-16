import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { message } from '../l10n/strings';
import { FREE_NEVER_SUBSCRIBED, json, problem, scriptedBilling } from './billing_transport';
import { renderApp } from './render';

const PAYMENT_ISSUE = { ...FREE_NEVER_SUBSCRIBED, plan: 'free', payment_issue: true };

describe('payment issue banner (LGN-215)', () => {
  it('Happy path: a failed renewal shows the banner on every signed-in screen, linking to Plans', async () => {
    const script = scriptedBilling({ plans: [json(PAYMENT_ISSUE)] });
    renderApp('/account', { client: script.client });

    const banner = await screen.findByRole('status');
    expect(banner).toHaveTextContent(message('banner.paymentIssue'));

    const link = screen.getByRole('link', { name: message('banner.paymentAction') });
    expect(link).toHaveAttribute('href', '/plans');
  });

  it('Edge: an account with no payment issue shows no banner', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/account', { client: script.client });

    expect(
      await screen.findByRole('heading', { name: message('account.title') }),
    ).toBeInTheDocument();
    expect(screen.queryByText(message('banner.paymentIssue'))).toBeNull();
  });

  it('Error: a failed plan read shows no banner and the screen still renders', async () => {
    const script = scriptedBilling({ plans: [problem('INTERNAL_ERROR', 'boom', 500)] });
    renderApp('/account', { client: script.client });

    expect(await screen.findByTestId('account-failure')).toBeInTheDocument();
    expect(screen.queryByText(message('banner.paymentIssue'))).toBeNull();
  });
});

describe('plans in the navigation (LGN-215)', () => {
  it('Happy path: the navigation links to the Plans screen', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/account', { client: script.client });

    expect(await screen.findByRole('link', { name: message('nav.plans') })).toHaveAttribute(
      'href',
      '/plans',
    );
  });
});
