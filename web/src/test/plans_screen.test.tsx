import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type * as RouterModule from 'react-router-dom';
import { redirectTo } from '../billing/redirect';
import { message, messageWith } from '../l10n/strings';
import { CHECKOUT_CONFIRM_LIMIT_MS } from '../plans/plan_view';
import type { Route } from './billing_transport';
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

const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('../billing/redirect', () => ({ redirectTo: vi.fn() }));

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof RouterModule>();
  return { ...actual, useNavigate: () => navigateSpy };
});

const ELIGIBLE = {
  ...FREE_NEVER_SUBSCRIBED,
  plan: 'paid',
  subscription_status: 'active',
  refund_eligible: true,
};

const COMP_ACTIVE = {
  ...PAID_ACTIVE,
  subscription_provider: 'comp',
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
  navigateSpy.mockClear();
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

    const busy = screen.getByRole('button', { name: message('plans.subscribe') });
    expect(busy).toHaveAttribute('aria-busy', 'true');
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

  it('Edge: a courtesy subscription has no Manage subscription action', async () => {
    const script = scriptedBilling({ plans: [json(COMP_ACTIVE)] });
    renderApp('/plans', { client: script.client });

    expect(
      await screen.findByRole('heading', { level: 2, name: message('account.planPaid') }),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('plans.manage') })).toBeNull();
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

describe('plans screen: courtesy plan', () => {
  it('Happy path: a courtesy plan is labelled as granted and carries no price', async () => {
    const script = scriptedBilling({ plans: [json(COMP_ACTIVE)] });
    renderApp('/plans', { client: script.client });

    expect(
      await screen.findByText(
        messageWith('plans.currentComp', { plan: message('account.planPaid') }),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(message('plans.compNote'))).toBeInTheDocument();
    expect(screen.queryByText(message('plans.priceAmount'))).toBeNull();
    expect(screen.queryByText(message('plans.priceNote'))).toBeNull();
    expect(screen.queryByText(message('plans.paidDescription'))).toBeNull();
  });

  it('Happy path: a courtesy grant with an end date names the access end, not a renewing period', async () => {
    const script = scriptedBilling({ plans: [json(COMP_ACTIVE)] });
    renderApp('/plans', { client: script.client });

    const date = new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(
      new Date(COMP_ACTIVE.current_period_end),
    );
    expect(
      await screen.findByText(messageWith('plans.compPeriodEnd', { date })),
    ).toBeInTheDocument();
    expect(screen.queryByText(messageWith('plans.periodEnd', { date }))).toBeNull();
  });

  it('Edge: a courtesy grant without an end date says it has none', async () => {
    const script = scriptedBilling({
      plans: [json({ ...COMP_ACTIVE, current_period_end: null })],
    });
    renderApp('/plans', { client: script.client });

    expect(await screen.findByText(message('plans.compNoEnd'))).toBeInTheDocument();
  });

  it('Edge: a card subscription keeps its price and payment copy', async () => {
    const script = scriptedBilling({ plans: [json(PAID_ACTIVE)] });
    renderApp('/plans', { client: script.client });

    expect(await screen.findByText(message('plans.priceAmount'))).toBeInTheDocument();
    expect(screen.getByText(message('plans.priceNote'))).toBeInTheDocument();
    expect(screen.getByText(message('plans.paidDescription'))).toBeInTheDocument();
    expect(screen.queryByText(message('plans.compNote'))).toBeNull();
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

    expect(await screen.findByText(message('plans.refundFailed'))).toBeInTheDocument();
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

  it('Error: a failed plan read shows the shared error state and retries with one more read', async () => {
    const script = scriptedBilling({
      plans: [problem('INTERNAL_ERROR', 'boom', 500), json(FREE_NEVER_SUBSCRIBED)],
    });
    const user = userEvent.setup();
    renderApp('/plans', { client: script.client });

    expect(await screen.findByRole('alert')).toHaveTextContent(message('plan.loadFailed'));
    await user.click(screen.getByRole('button', { name: message('common.retry') }));

    expect(
      await screen.findByRole('button', { name: message('plans.subscribe') }),
    ).toBeInTheDocument();
    expect(script.planReads()).toBe(2);
  });
});

describe('plans screen layout and standing conditions (DESIGN.md)', () => {
  it('Happy path: the page title is the shared page header', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/plans', { client: script.client });

    expect(
      await screen.findByRole('heading', { level: 1, name: message('plans.title') }),
    ).toBeInTheDocument();
  });

  it('Happy path: an ending subscription is a warning notice and the period end an info notice', async () => {
    const script = scriptedBilling({
      plans: [json({ ...PAID_ACTIVE, cancel_at_period_end: true })],
    });
    renderApp('/plans', { client: script.client });

    const cancellation = (await screen.findByText(message('plans.cancelAtPeriodEnd'))).closest(
      '.lg-notice',
    );
    expect(cancellation).toHaveClass('lg-notice--warning');
    expect(cancellation).toHaveAttribute('role', 'status');

    const date = new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(
      new Date(PAID_ACTIVE.current_period_end),
    );
    const period = screen.getByText(messageWith('plans.periodEnd', { date })).closest('.lg-notice');
    expect(period).toHaveClass('lg-notice--info');
    expect(period).toHaveAttribute('role', 'status');
  });

  it('Edge: an account that never subscribed gets one muted line, not a notice', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/plans', { client: script.client });

    const line = await screen.findByText(message('plans.neverSubscribed'));
    expect(line).toHaveClass('lg-plans__text');
    expect(line.closest('.lg-notice')).toBeNull();
  });
});

describe('plans screen: paid advantages (LGN-340)', () => {
  async function paidCard(): Promise<HTMLElement> {
    const heading = await screen.findByRole('heading', { name: message('account.planPaid') });
    const card = heading.closest('article');
    expect(card).not.toBeNull();
    return card as HTMLElement;
  }

  it('Happy path: a free account sees No ads as a Paid plan advantage', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/plans', { client: script.client });

    const paid = await paidCard();
    const advantages = within(paid).getByRole('list', { name: message('plans.paidAdvantages') });
    expect(within(advantages).getByText(message('plans.paidNoAds'))).toBeInTheDocument();
  });

  it('Edge: the Free plan card does not list the Paid advantages', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/plans', { client: script.client });

    const heading = await screen.findByRole('heading', { name: message('account.planFree') });
    const free = heading.closest('article');
    expect(free).not.toBeNull();
    expect(within(free as HTMLElement).queryByText(message('plans.paidNoAds'))).toBeNull();
  });

  it('Edge: a paid account still sees No ads in its current plan card', async () => {
    const script = scriptedBilling({ plans: [json(PAID_ACTIVE)] });
    renderApp('/plans', { client: script.client });

    const paid = await paidCard();
    expect(
      within(paid).getByText(messageWith('plans.current', { plan: message('account.planPaid') })),
    ).toBeInTheDocument();
    expect(within(paid).getByText(message('plans.paidNoAds'))).toBeInTheDocument();
  });
});

describe('plans screen: return from checkout (LGN-256)', () => {
  beforeEach(() => {
    vi.useFakeTimers({
      toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'Date'],
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  async function advance(ms: number) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ms);
    });
  }

  const ACTIVATED_COPY = messageWith('plans.checkoutActivated', {
    plan: message('account.planPaid'),
  });

  function expectCheckoutParamRemoved() {
    expect(navigateSpy).toHaveBeenCalledWith({ pathname: '/plans', search: '' }, { replace: true });
  }

  it('Happy path: a Free plan that turns paid on a later poll shows the notice and drops the param', async () => {
    const script = scriptedBilling({
      plans: [json(FREE_NEVER_SUBSCRIBED), json(FREE_NEVER_SUBSCRIBED), json(PAID_ACTIVE)],
    });
    renderApp('/plans?checkout=success', { client: script.client });

    await advance(0);
    expect(screen.getByText(message('plans.checkoutConfirming'))).toBeInTheDocument();
    expect(screen.queryByText(ACTIVATED_COPY)).toBeNull();

    await advance(2000);
    expect(screen.getByText(message('plans.checkoutConfirming'))).toBeInTheDocument();

    await advance(2000);
    await advance(1);
    expect(screen.getByText(ACTIVATED_COPY)).toBeInTheDocument();
    expect(screen.queryByText(message('plans.checkoutConfirming'))).toBeNull();
    expect(script.planReads()).toBe(3);
    expectCheckoutParamRemoved();
  });

  it('Edge: a delayed webhook keeps confirming and never presents Free as the final state', async () => {
    const script = scriptedBilling({ plans: [json(FREE_NEVER_SUBSCRIBED)] });
    renderApp('/plans?checkout=success', { client: script.client });

    await advance(0);
    await advance(4000);

    expect(screen.getByText(message('plans.checkoutConfirming'))).toBeInTheDocument();
    expect(
      screen.queryByText(messageWith('plans.current', { plan: message('account.planFree') })),
    ).toBeNull();
    expect(screen.queryByRole('button', { name: message('plans.subscribe') })).toBeNull();
    expect(screen.queryByText(ACTIVATED_COPY)).toBeNull();
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('Edge: the wait limit shows the processing message and Check again reads the plan once more', async () => {
    const plans: Route[] = [json(FREE_NEVER_SUBSCRIBED)];
    const script = scriptedBilling({ plans });
    renderApp('/plans?checkout=success', { client: script.client });

    await advance(0);
    await advance(CHECKOUT_CONFIRM_LIMIT_MS);

    expect(screen.getByText(message('plans.checkoutProcessing'))).toBeInTheDocument();
    expect(screen.queryByText(message('plans.checkoutConfirming'))).toBeNull();
    expectCheckoutParamRemoved();

    const readsBefore = script.planReads();
    plans.push(json(PAID_ACTIVE));
    fireEvent.click(screen.getByRole('button', { name: message('plans.checkoutCheckAgain') }));
    await advance(0);

    expect(script.planReads()).toBe(readsBefore + 1);
    expect(screen.getByText(ACTIVATED_COPY)).toBeInTheDocument();
    expect(screen.queryByText(message('plans.checkoutProcessing'))).toBeNull();
  });

  it('Edge: checkout=canceled keeps the previous plan, shows no message and drops the param', async () => {
    const script = scriptedBilling({ plans: [json(PAID_ACTIVE)] });
    renderApp('/plans?checkout=canceled', { client: script.client });

    await advance(0);

    expect(
      screen.getByText(messageWith('plans.current', { plan: message('account.planPaid') })),
    ).toBeInTheDocument();
    expect(screen.queryByText(message('plans.checkoutConfirming'))).toBeNull();
    expect(screen.queryByText(message('plans.checkoutProcessing'))).toBeNull();
    expect(screen.queryByText(ACTIVATED_COPY)).toBeNull();
    expectCheckoutParamRemoved();
  });

  it('Happy path: an account already paid on the first read shows the notice directly', async () => {
    const script = scriptedBilling({ plans: [json(PAID_ACTIVE)] });
    renderApp('/plans?checkout=success', { client: script.client });

    await advance(0);

    expect(screen.getByText(ACTIVATED_COPY)).toBeInTheDocument();
    expect(screen.queryByText(message('plans.checkoutConfirming'))).toBeNull();
    expect(script.planReads()).toBe(1);
    expectCheckoutParamRemoved();
    expect(redirectTo).not.toHaveBeenCalled();
  });
});
