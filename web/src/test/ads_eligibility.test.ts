import { describe, expect, it } from 'vitest';
import { adsEligible } from '../ads/eligibility';
import type { AdsConfig } from '../ads/ads_config';
import type { BillingStatus, PlanUsage } from '../api/models/plan';

const CLIENT = 'ca-pub-1234567890123456';

const CONFIG: AdsConfig = { client: CLIENT, slots: { catalog: '1', campaigns: '2' } };

const BILLING: BillingStatus = {
  subscriptionStatus: null,
  subscriptionProvider: null,
  currentPeriodEnd: null,
  cancelAtPeriodEnd: false,
  paymentIssue: false,
  refundEligible: false,
  refundState: null,
};

function plan(overrides: Partial<PlanUsage> = {}, billing: Partial<BillingStatus> = {}): PlanUsage {
  return {
    plan: 'free',
    sheets: {},
    consentAt: null,
    consentVersion: null,
    llmConsentRequired: false,
    activeGenerationId: null,
    billing: { ...BILLING, ...billing },
    ...overrides,
  };
}

function eligible(
  config: AdsConfig | null,
  signedIn: boolean,
  usage: PlanUsage | undefined,
): boolean {
  return adsEligible({ config, signedIn, plan: usage });
}

describe('ad eligibility', () => {
  it('Happy path: a signed-in Free account is eligible', () => {
    expect(eligible(CONFIG, true, plan())).toBe(true);
  });

  it('Edge: a paid account is never eligible', () => {
    expect(eligible(CONFIG, true, plan({ plan: 'paid' }))).toBe(false);
  });

  it('Edge: an active subscription on the free plan is paid and therefore not eligible', () => {
    expect(eligible(CONFIG, true, plan({}, { subscriptionStatus: 'active' }))).toBe(false);
  });

  it('Edge: a plan that is still loading or absent is not eligible', () => {
    expect(eligible(CONFIG, true, undefined)).toBe(false);
  });

  it('Edge: a logged-out visitor is never eligible', () => {
    expect(eligible(CONFIG, false, undefined)).toBe(false);
    expect(eligible(CONFIG, false, plan())).toBe(false);
  });

  it('Edge: no ads configuration means no surface is eligible', () => {
    expect(eligible(null, false, undefined)).toBe(false);
    expect(eligible(null, true, plan())).toBe(false);
  });
});
