import type { PlanUsage } from '../api/models/plan';
import { message, messageWith } from '../l10n/strings';

export const REFUND_POLL_MS = 5000;

export const CHECKOUT_POLL_MS = 2000;

export const CHECKOUT_CONFIRM_LIMIT_MS = 30000;

export type CheckoutReturn = 'success' | 'canceled' | null;

export type CheckoutPhase = 'idle' | 'confirming' | 'activated' | 'processing';

export function refundPollingInterval(plan: PlanUsage | undefined): number | false {
  return plan?.billing.refundState === 'requested' ? REFUND_POLL_MS : false;
}

export function checkoutReturn(value: string | null): CheckoutReturn {
  return value === 'success' || value === 'canceled' ? value : null;
}

export function isPaidPlan(plan: PlanUsage | undefined): boolean {
  return (
    plan !== undefined && (plan.plan !== 'free' || plan.billing.subscriptionStatus === 'active')
  );
}

export function checkoutPhase(
  returned: CheckoutReturn,
  plan: PlanUsage | undefined,
  timedOut: boolean,
): CheckoutPhase {
  if (returned !== 'success') {
    return 'idle';
  }
  if (isPaidPlan(plan)) {
    return 'activated';
  }
  return timedOut ? 'processing' : 'confirming';
}

export function checkoutPollingInterval(
  phase: CheckoutPhase,
  plan: PlanUsage | undefined,
): number | false {
  if (phase === 'confirming') {
    return CHECKOUT_POLL_MS;
  }
  if (phase === 'processing') {
    return false;
  }
  return refundPollingInterval(plan);
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
