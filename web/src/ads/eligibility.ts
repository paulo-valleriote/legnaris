import type { PlanUsage } from '../api/models/plan';
import { isPaidPlan } from '../plans/plan_view';
import type { AdSurface, AdsConfig } from './ads_config';

export interface AdsEligibilityInput {
  readonly config: AdsConfig | null;
  readonly signedIn: boolean;
  readonly plan: PlanUsage | undefined;
  readonly surface: AdSurface;
}

export function adsEligible({ config, signedIn, plan, surface }: AdsEligibilityInput): boolean {
  if (config === null) {
    return false;
  }

  if (!signedIn) {
    return surface === 'landing';
  }

  if (plan === undefined) {
    return false;
  }

  return !isPaidPlan(plan);
}
