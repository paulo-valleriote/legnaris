import type { PlanUsage } from '../api/models/plan';
import { message, messageWith } from '../l10n/strings';

export const REFUND_POLL_MS = 5000;

export function refundPollingInterval(plan: PlanUsage | undefined): number | false {
  return plan?.billing.refundState === 'requested' ? REFUND_POLL_MS : false;
}

export function planName(plan: string): string {
  if (plan === 'free') {
    return message('account.planFree');
  }
  if (plan === 'paid') {
    return message('account.planPaid');
  }
  return messageWith('account.planOther', { plan });
}
