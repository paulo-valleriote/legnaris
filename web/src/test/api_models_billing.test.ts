import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure } from '../api/errors';
import { parseRedirectUrl } from '../api/models/billing';
import { parsePlanUsage } from '../api/models/plan';

const PLAN_BASE = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
};

describe('billing redirect model (LGN-215)', () => {
  it('Happy path: decodes the provider URL of a checkout or portal answer', () => {
    expect(parseRedirectUrl({ url: 'https://checkout.stripe.com/c/pay/abc' })).toBe(
      'https://checkout.stripe.com/c/pay/abc',
    );
  });

  it('Edge: an answer carrying an extra expiry field still yields its URL', () => {
    expect(
      parseRedirectUrl({
        url: 'https://billing.example.test/p',
        expires_at: '2026-11-17T10:00:00Z',
      }),
    ).toBe('https://billing.example.test/p');
  });

  it.each([
    { name: 'a non-object payload', data: 'https://checkout.stripe.com/x' },
    { name: 'a null payload', data: null },
    { name: 'a payload without a url', data: { expires_at: '2026-11-17T10:00:00Z' } },
    { name: 'a url that is not a string', data: { url: 42 } },
    { name: 'an empty url', data: { url: '' } },
    { name: 'a plain http url', data: { url: 'http://checkout.stripe.com/x' } },
    { name: 'a script url', data: { url: ['javas', 'cript:alert(1)'].join('') } },
    { name: 'a data url', data: { url: 'data:text/html,hi' } },
    { name: 'a relative url', data: { url: '/billing/checkout' } },
    { name: 'an unparseable url', data: { url: 'https://' } },
  ])('Error: $name is refused as an unreadable answer', ({ data }) => {
    expect(() => parseRedirectUrl(data)).toThrow(DefensiveParseFailure);
  });
});

describe('plan billing status (LGN-215)', () => {
  it('Happy path: decodes the subscription, payment issue and refund fields', () => {
    const usage = parsePlanUsage({
      ...PLAN_BASE,
      plan: 'paid',
      subscription_status: 'active',
      current_period_end: '2026-12-01T00:00:00Z',
      cancel_at_period_end: true,
      payment_issue: false,
      refund_eligible: true,
      refund_state: null,
    });

    expect(usage.billing).toEqual({
      subscriptionStatus: 'active',
      currentPeriodEnd: '2026-12-01T00:00:00Z',
      cancelAtPeriodEnd: true,
      paymentIssue: false,
      refundEligible: true,
      refundState: null,
    });
  });

  it('Edge: an answer without any billing field reads as never subscribed with no issue', () => {
    const usage = parsePlanUsage(PLAN_BASE);

    expect(usage.billing).toEqual({
      subscriptionStatus: null,
      currentPeriodEnd: null,
      cancelAtPeriodEnd: false,
      paymentIssue: false,
      refundEligible: false,
      refundState: null,
    });
  });

  it('Edge: explicit nulls read the same as absent billing fields', () => {
    const usage = parsePlanUsage({
      ...PLAN_BASE,
      subscription_status: null,
      current_period_end: null,
      cancel_at_period_end: null,
      payment_issue: null,
      refund_eligible: null,
      refund_state: null,
    });

    expect(usage.billing.subscriptionStatus).toBeNull();
    expect(usage.billing.paymentIssue).toBe(false);
    expect(usage.billing.refundEligible).toBe(false);
  });

  it.each(['pending', 'active', 'past_due', 'canceled'])(
    'Edge: the subscription status "%s" is accepted',
    (status) => {
      expect(
        parsePlanUsage({ ...PLAN_BASE, subscription_status: status }).billing.subscriptionStatus,
      ).toBe(status);
    },
  );

  it.each(['requested', 'refunded', 'canceled', 'failed'])(
    'Edge: the refund state "%s" is accepted',
    (state) => {
      expect(parsePlanUsage({ ...PLAN_BASE, refund_state: state }).billing.refundState).toBe(state);
    },
  );

  it.each([
    { name: 'an unknown subscription status', field: { subscription_status: 'paused' } },
    { name: 'a numeric subscription status', field: { subscription_status: 1 } },
    { name: 'an unknown refund state', field: { refund_state: 'pending' } },
    { name: 'a non-boolean payment issue', field: { payment_issue: 'yes' } },
    { name: 'a non-boolean refund eligibility', field: { refund_eligible: 1 } },
    { name: 'a non-boolean cancel flag', field: { cancel_at_period_end: 'true' } },
    { name: 'a period end that is not a timestamp', field: { current_period_end: 'soon' } },
    { name: 'a period end that is not a string', field: { current_period_end: 5 } },
  ])('Error: $name is refused as an unreadable plan', ({ field }) => {
    expect(() => parsePlanUsage({ ...PLAN_BASE, ...field })).toThrow(DefensiveParseFailure);
  });
});
