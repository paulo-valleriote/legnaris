import { describe, expect, it } from 'vitest';
import type { PlanUsage } from '../api/models/plan';
import { message, messageWith } from '../l10n/strings';
import { planName, refundPollingInterval, REFUND_POLL_MS } from '../plans/plan_view';

function planWithRefund(refundState: PlanUsage['billing']['refundState']): PlanUsage {
  return {
    plan: 'paid',
    sheets: {},
    consentAt: null,
    consentVersion: null,
    trialRemaining: undefined,
    llmConsentRequired: false,
    lastFreeGeneration: false,
    activeGenerationId: null,
    billing: {
      subscriptionStatus: 'active',
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
    expect(refundPollingInterval(planWithRefund('requested'))).toBe(REFUND_POLL_MS);
    expect(REFUND_POLL_MS).toBeGreaterThan(0);
  });

  it.each([
    { name: 'no plan read yet', plan: undefined },
    { name: 'no refund on the account', plan: planWithRefund(null) },
    { name: 'a refund already completed', plan: planWithRefund('refunded') },
    { name: 'a refund already canceled', plan: planWithRefund('canceled') },
    { name: 'a refund that failed', plan: planWithRefund('failed') },
  ])('Edge: $name is not polled', ({ plan }) => {
    expect(refundPollingInterval(plan)).toBe(false);
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
