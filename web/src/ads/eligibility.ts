import type { PlanUsage } from '../api/models/plan';
import { isPaidPlan } from '../plans/plan_view';
import type { AdsConfig } from './ads_config';

export interface AdsEligibilityInput {
  readonly config: AdsConfig | null;
  readonly signedIn: boolean;
  readonly plan: PlanUsage | undefined;
}

export function adsEligible({ config, signedIn, plan }: AdsEligibilityInput): boolean {
  if (config === null) {
    return false;
  }

  if (!signedIn) {
    return false;
  }

  if (plan === undefined) {
    return false;
  }

  return !isPaidPlan(plan);
}
