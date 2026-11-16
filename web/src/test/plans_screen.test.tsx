import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { redirectTo } from '../billing/redirect';
import { message, messageWith } from '../l10n/strings';
import {
  ACCEPTED,
  CHECKOUT_URL,
  FREE_NEVER_SUBSCRIBED,
  PAID_ACTIVE,
  PORTAL_URL,
  json,
  problem,
  scriptedBilling,
} from './billing_transport';
import { renderApp } from './render';

vi.mock('../billing/redirect', () => ({ redirectTo: vi.fn() }));

const ELIGIBLE = {
  ...FREE_NEVER_SUBSCRIBED,
  plan: 'paid',
  subscription_status: 'active',
  refund_eligible: true,
};

function deferredRoute() {
  let settle: (response: { status: number; body: string }) => void = () => undefined;
  const pending = new Promise<{ status: number; body: string }>((resolve) => {
    settle = resolve;
  });
  return { route: pending, settle };
}

beforeEach(() => {
  vi.mocked(redirectTo).mockClear();
});

describe('plans screen: checkout (LGN-215)', () => {
  it('Happy path: a free account sees its current Plan and subscribing redirects to the provider page', async () => {
    const script = scriptedBilling({
      plans: [json(FREE_NEVER_SUBSCRIBED)],
      checkout: json({ url: CHECKOUT_URL, expires_at: '2026-11-17T10:00:00Z' }),
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    expect(
      await screen.findByText(messageWith('plans.current', { plan: message('account.planFree') })),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('plans.subscribe') }));

    expect(script.posts('/billing/checkout')).toHaveLength(1);
    expect(script.posts('/billing/checkout')[0]?.body).toBeUndefined();
    expect(redirectTo).toHaveBeenCalledWith(CHECKOUT_URL);
  });

  it('Edge: while the checkout is being opened the button is busy and a second press sends nothing', async () => {
    const pending = deferredRoute();
    const script = scriptedBilling({
      plans: [json(FREE_NEVER_SUBSCRIBED)],
      checkout: pending.route,
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    const subscribe = await screen.findByRole('button', { name: message('plans.subscribe') });
    await user.click(subscribe);

    const busy = screen.getByRole('button', { name: message('plans.subscribing') });
    expect(busy).toBeDisabled();
    await user.click(busy);
    expect(script.posts('/billing/checkout')).toHaveLength(1);

    await act(async () => {
      pending.settle(json({ url: CHECKOUT_URL }));
    });
    expect(redirectTo).toHaveBeenCalledTimes(1);
  });

  it('Edge: a free account that never subscribed is told so and has no manage action', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/plans', { client: script.client });

    expect(await screen.findByText(message('plans.neverSubscribed'))).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('plans.manage') })).toBeNull();
  });

  it('Edge: ALREADY_SUBSCRIBED shows the paid Plan and reloads the plan instead of redirecting', async () => {
    const script = scriptedBilling({
      plans: [json(FREE_NEVER_SUBSCRIBED), json(PAID_ACTIVE)],
      checkout: problem('ALREADY_SUBSCRIBED', 'active', 409),
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.subscribe') }));

    expect(await screen.findByText(message('api.error.alreadySubscribed'))).toBeInTheDocument();
    expect(redirectTo).not.toHaveBeenCalled();
    expect(script.planReads()).toBe(2);
    expect(
      await screen.findByRole('button', { name: message('plans.manage') }),
    ).toBeInTheDocument();
  });

  it('Error: BILLING_UNAVAILABLE shows the mapped copy and does not redirect', async () => {
    const script = scriptedBilling({
      plans: [json(FREE_NEVER_SUBSCRIBED)],
      checkout: problem('BILLING_UNAVAILABLE', 'down', 503),
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.subscribe') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.billingUnavailable'),
    );
    expect(redirectTo).not.toHaveBeenCalled();
    expect(script.posts('/billing/checkout')).toHaveLength(1);
  });
});

describe('plans screen: portal (LGN-215)', () => {
  it('Happy path: an account with a subscription opens the billing page from Manage subscription', async () => {
    const script = scriptedBilling({
      plans: [json(PAID_ACTIVE)],
      portal: json({ url: PORTAL_URL }),
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.manage') }));

    expect(script.posts('/billing/portal')).toHaveLength(1);
    expect(redirectTo).toHaveBeenCalledWith(PORTAL_URL);
    expect(screen.queryByRole('button', { name: message('plans.subscribe') })).toBeNull();
  });

  it('Error: NO_SUBSCRIPTION from the portal shows the mapped copy and does not redirect', async () => {
    const script = scriptedBilling({
      plans: [json(PAID_ACTIVE), json(FREE_NEVER_SUBSCRIBED)],
      portal: problem('NO_SUBSCRIPTION', 'none', 404),
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.manage') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.noSubscription'));
    expect(redirectTo).not.toHaveBeenCalled();
    expect(script.planReads()).toBe(2);
  });
});

describe('plans screen: refund (LGN-215)', () => {
  it('Edge: an account outside the refund window sees no refund action', async () => {
    const script = scriptedBilling({
      plans: [json({ ...PAID_ACTIVE, refund_eligible: false })],
    });
    renderApp('/plans', { client: script.client });

    expect(
      await screen.findByRole('button', { name: message('plans.manage') }),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('plans.refundAction') })).toBeNull();
  });

  it('Happy path: confirming the refund sends one request and shows the in-progress state from the reload', async () => {
    const script = scriptedBilling({
      plans: [
        json(ELIGIBLE),
        json({ ...ELIGIBLE, refund_eligible: false, refund_state: 'requested' }),
      ],
      refund: ACCEPTED,
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.refundAction') }));
    const dialog = screen.getByRole('dialog', { name: message('plans.refundTitle') });
    await user.click(within(dialog).getByRole('button', { name: message('plans.refundConfirm') }));

    expect(script.posts('/billing/refund')).toHaveLength(1);
    expect(await screen.findByText(message('plans.refundRequested'))).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByRole('button', { name: message('plans.refundAction') })).toBeNull();
  });

  it('Edge: keeping the Plan in the dialog sends nothing', async () => {
    const script = scriptedBilling({ plans: [json(ELIGIBLE)] });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.refundAction') }));
    await user.click(screen.getByRole('button', { name: message('plans.refundKeep') }));

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(script.posts('/billing/refund')).toHaveLength(0);
  });

  it('Edge: Escape closes the refund dialog without sending anything', async () => {
    const script = scriptedBilling({ plans: [json(ELIGIBLE)] });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.refundAction') }));
    await user.keyboard('{Escape}');

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(script.posts('/billing/refund')).toHaveLength(0);
  });

  it('Error: a refund that is no longer eligible shows the mapped copy inside the dialog', async () => {
    const script = scriptedBilling({
      plans: [json(ELIGIBLE)],
      refund: problem('REFUND_NOT_ELIGIBLE', 'window expired', 409),
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    await user.click(await screen.findByRole('button', { name: message('plans.refundAction') }));
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', {
        name: message('plans.refundConfirm'),
      }),
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.refundNotEligible'),
    );
    expect(script.posts('/billing/refund')).toHaveLength(1);
  });

  it('Happy path: a completed refund is stated as complete', async () => {
    const script = scriptedBilling({
      plans: [json({ ...FREE_NEVER_SUBSCRIBED, refund_state: 'refunded' })],
    });
    renderApp('/plans', { client: script.client });

    expect(await screen.findByText(message('plans.refundCompleted'))).toBeInTheDocument();
  });

  it('Error: a failed refund says the right is back and offers the refund again while eligible', async () => {
    const script = scriptedBilling({
      plans: [json({ ...ELIGIBLE, refund_state: 'failed' })],
    });
    renderApp('/plans', { client: script.client });

    expect(await screen.findByRole('alert')).toHaveTextContent(message('plans.refundFailed'));
    expect(screen.getByRole('button', { name: message('plans.refundAction') })).toBeInTheDocument();
  });
});

describe('plans screen states (LGN-215)', () => {
  it('Edge: while the plan loads the screen shows the loading line and no actions', () => {
    const script = scriptedBilling({ plans: [new Promise(() => undefined)] });
    renderApp('/plans', { client: script.client });

    expect(screen.getByText(message('plans.loading'))).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('plans.subscribe') })).toBeNull();
  });

  it('Error: a failed plan read shows the mapped copy and retries with one more read', async () => {
    const script = scriptedBilling({
      plans: [problem('INTERNAL_ERROR', 'boom', 500), json(FREE_NEVER_SUBSCRIBED)],
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.serverError'));
    await user.click(screen.getByRole('button', { name: message('plans.retry') }));

    expect(
      await screen.findByRole('button', { name: message('plans.subscribe') }),
    ).toBeInTheDocument();
    expect(script.planReads()).toBe(2);
  });
});
