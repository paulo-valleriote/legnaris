import { describe, expect, it } from 'vitest';
import type { PlanUsage } from '../api/models/plan';
import { message, messageWith } from '../l10n/strings';
import {
  CHECKOUT_CONFIRM_LIMIT_MS,
  CHECKOUT_POLL_MS,
  checkoutPhase,
  checkoutPollingInterval,
  checkoutReturn,
  isPaidPlan,
  planName,
  refundPollingInterval,
  REFUND_POLL_MS,
} from '../plans/plan_view';

function planUsage(
  plan = 'paid',
  subscriptionStatus: PlanUsage['billing']['subscriptionStatus'] = 'active',
  refundState: PlanUsage['billing']['refundState'] = null,
): PlanUsage {
  return {
    plan,
    sheets: {},
    consentAt: null,
    consentVersion: null,
    llmConsentRequired: false,
    activeGenerationId: null,
    billing: {
      subscriptionStatus,
      subscriptionProvider: null,
      currentPeriodEnd: null,
      cancelAtPeriodEnd: false,
      paymentIssue: false,
      refundEligible: false,
      refundState,
    },
  };
}

describe('refund polling (LGN-215)', () => {
  it('Happy path: a requested refund is re-read on a short interval until it completes', () => {
    expect(refundPollingInterval(planUsage('paid', 'active', 'requested'))).toBe(REFUND_POLL_MS);
    expect(REFUND_POLL_MS).toBeGreaterThan(0);
  });

  it.each([
    { name: 'no plan read yet', plan: undefined },
    { name: 'no refund on the account', plan: planUsage('paid', 'active', null) },
    { name: 'a refund already completed', plan: planUsage('paid', 'active', 'refunded') },
    { name: 'a refund already canceled', plan: planUsage('paid', 'active', 'canceled') },
    { name: 'a refund that failed', plan: planUsage('paid', 'active', 'failed') },
  ])('Edge: $name is not polled', ({ plan }) => {
    expect(refundPollingInterval(plan)).toBe(false);
  });
});

describe('checkout return parsing (LGN-256)', () => {
  it.each([
    { name: 'a success return', value: 'success', expected: 'success' },
    { name: 'a canceled return', value: 'canceled', expected: 'canceled' },
    { name: 'no parameter', value: null, expected: null },
    { name: 'an empty value', value: '', expected: null },
    { name: 'an unknown value', value: 'pending', expected: null },
  ])('Happy path: $name resolves to $expected', ({ value, expected }) => {
    expect(checkoutReturn(value)).toBe(expected);
  });
});

describe('paid plan detection (LGN-256)', () => {
  it('Happy path: the paid Plan and any active subscription count as paid', () => {
    expect(isPaidPlan(planUsage('paid', 'active'))).toBe(true);
    expect(isPaidPlan(planUsage('free', 'active'))).toBe(true);
  });

  it.each([
    { name: 'the plan has not been read', plan: undefined },
    { name: 'the account is Free and never subscribed', plan: planUsage('free', null) },
    { name: 'the account is Free with a pending subscription', plan: planUsage('free', 'pending') },
    { name: 'the account is Free after a failed renewal', plan: planUsage('free', 'past_due') },
  ])('Edge: $name is not paid', ({ plan }) => {
    expect(isPaidPlan(plan)).toBe(false);
  });

  it('Edge: an unknown non-Free Plan counts as paid', () => {
    expect(isPaidPlan(planUsage('team', null))).toBe(true);
  });
});

describe('checkout return phase (LGN-256)', () => {
  it('Happy path: a success return with a Free plan is confirming until paid', () => {
    expect(checkoutPhase('success', planUsage('free', null), false)).toBe('confirming');
    expect(checkoutPhase('success', planUsage('paid', 'active'), false)).toBe('activated');
  });

  it('Happy path: a paid Plan after the wait limit is still activation, not processing', () => {
    expect(checkoutPhase('success', planUsage('paid', 'active'), true)).toBe('activated');
  });

  it('Edge: a success return at the wait limit with a Free plan is processing', () => {
    expect(checkoutPhase('success', planUsage('free', null), true)).toBe('processing');
  });

  it.each([
    { name: 'a canceled return', returned: 'canceled' as const },
    { name: 'no return at all', returned: null },
  ])('Edge: $name never leaves the normal plan state', ({ returned }) => {
    expect(checkoutPhase(returned, planUsage('free', null), false)).toBe('idle');
    expect(checkoutPhase(returned, planUsage('free', null), true)).toBe('idle');
  });
});

describe('checkout polling interval (LGN-256)', () => {
  it('Happy path: confirming re-reads the plan every two seconds for up to thirty seconds', () => {
    expect(checkoutPollingInterval('confirming', planUsage('free', null))).toBe(CHECKOUT_POLL_MS);
    expect(CHECKOUT_POLL_MS).toBe(2000);
    expect(CHECKOUT_CONFIRM_LIMIT_MS).toBe(30000);
  });

  it('Edge: confirming outranks the refund poll until the plan resolves', () => {
    expect(checkoutPollingInterval('confirming', planUsage('paid', 'active', 'requested'))).toBe(
      CHECKOUT_POLL_MS,
    );
  });

  it('Edge: processing stops polling so only Check again reads the plan', () => {
    expect(checkoutPollingInterval('processing', planUsage('free', null))).toBe(false);
    expect(checkoutPollingInterval('processing', planUsage('paid', 'active', 'requested'))).toBe(
      false,
    );
  });

  it('Edge: an activated or idle screen keeps the refund poll and nothing else', () => {
    expect(checkoutPollingInterval('activated', planUsage('paid', 'active', 'requested'))).toBe(
      REFUND_POLL_MS,
    );
    expect(checkoutPollingInterval('activated', planUsage('paid', 'active', null))).toBe(false);
    expect(checkoutPollingInterval('idle', planUsage('paid', 'active', 'requested'))).toBe(
      REFUND_POLL_MS,
    );
    expect(checkoutPollingInterval('idle', planUsage('free', null))).toBe(false);
  });
});

describe('plan names (LGN-215)', () => {
  it('Happy path: the free and paid Plans carry their names', () => {
    expect(planName('free')).toBe(message('account.planFree'));
    expect(planName('paid')).toBe(message('account.planPaid'));
  });

  it('Edge: a Plan the client does not know is named as given', () => {
    expect(planName('team')).toBe(messageWith('account.planOther', { plan: 'team' }));
  });

  it('Edge: an empty Plan name is shown as an unknown Plan, never blank', () => {
    expect(planName('')).toBe(messageWith('account.planOther', { plan: '' }));
  });
});
