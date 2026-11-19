import { describe, expect, it } from 'vitest';
import { adsEligible } from '../ads/eligibility';
import type { AdSurface, AdsConfig } from '../ads/ads_config';
import type { BillingStatus, PlanUsage } from '../api/models/plan';

const CLIENT = 'ca-pub-1234567890123456';

const CONFIG: AdsConfig = { client: CLIENT, slots: { catalog: '1', campaigns: '2', landing: '3' } };

const BILLING: BillingStatus = {
  subscriptionStatus: null,
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
  surface: AdSurface,
): boolean {
  return adsEligible({ config, signedIn, plan: usage, surface });
}

describe('ad eligibility', () => {
  it('Happy path: a signed-in Free account is eligible on every surface', () => {
    for (const surface of ['catalog', 'campaigns', 'landing'] as const) {
      expect(eligible(CONFIG, true, plan(), surface)).toBe(true);
    }
  });

  it('Edge: a paid account is never eligible', () => {
    for (const surface of ['catalog', 'campaigns', 'landing'] as const) {
      expect(eligible(CONFIG, true, plan({ plan: 'paid' }), surface)).toBe(false);
    }
  });

  it('Edge: an active subscription on the free plan is paid and therefore not eligible', () => {
    expect(eligible(CONFIG, true, plan({}, { subscriptionStatus: 'active' }), 'catalog')).toBe(
      false,
    );
  });

  it('Edge: a plan that is still loading or absent is not eligible', () => {
    for (const surface of ['catalog', 'campaigns', 'landing'] as const) {
      expect(eligible(CONFIG, true, undefined, surface)).toBe(false);
    }
  });

  it('Happy path: logged out, only the landing surface is eligible', () => {
    expect(eligible(CONFIG, false, undefined, 'landing')).toBe(true);
  });

  it('Edge: logged out, the catalog and campaigns surfaces are not eligible', () => {
    expect(eligible(CONFIG, false, undefined, 'catalog')).toBe(false);
    expect(eligible(CONFIG, false, undefined, 'campaigns')).toBe(false);
  });

  it('Edge: no ads configuration means no surface is eligible', () => {
    for (const surface of ['catalog', 'campaigns', 'landing'] as const) {
      expect(eligible(null, false, undefined, surface)).toBe(false);
      expect(eligible(null, true, plan(), surface)).toBe(false);
    }
  });
});
