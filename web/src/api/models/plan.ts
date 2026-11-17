import { drift, isPlainObject, optionalString, requireNumber, requireString } from '../parse';

export const SHEET_KINDS = ['character', 'npc', 'enemy'] as const;

export type SheetKind = (typeof SHEET_KINDS)[number];

export const SUBSCRIPTION_STATUSES = ['pending', 'active', 'past_due', 'canceled'] as const;

export type SubscriptionStatus = (typeof SUBSCRIPTION_STATUSES)[number];

export const REFUND_STATES = ['requested', 'refunded', 'canceled', 'failed'] as const;

export type RefundState = (typeof REFUND_STATES)[number];

export interface BillingStatus {
  readonly subscriptionStatus: SubscriptionStatus | null;
  readonly currentPeriodEnd: string | null;
  readonly cancelAtPeriodEnd: boolean;
  readonly paymentIssue: boolean;
  readonly refundEligible: boolean;
  readonly refundState: RefundState | null;
}

export interface SheetUsage {
  readonly used: number;
  readonly limit: number;
}

export interface PlanUsage {
  readonly plan: string;
  readonly sheets: Readonly<Partial<Record<SheetKind, SheetUsage>>>;
  readonly consentAt: string | null;
  readonly consentVersion: string | null;
  readonly trialRemaining: number | undefined;
  readonly llmConsentRequired: boolean;
  readonly lastFreeGeneration: boolean;
  readonly activeGenerationId: string | null;
  readonly billing: BillingStatus;
  readonly ownedCampaigns?: SheetUsage;
}

function decodeSheetUsage(value: unknown, kind: string): SheetUsage {
  if (!isPlainObject(value)) {
    throw drift(`plan usage for "${kind}" is not an object`);
  }
  return { used: requireNumber(value, 'used'), limit: requireNumber(value, 'limit') };
}

export function parsePlanUsage(data: unknown): PlanUsage {
  if (!isPlainObject(data)) {
    throw drift('plan payload is not an object');
  }
  const rawSheets = data['sheets'];
  if (!isPlainObject(rawSheets)) {
    throw drift('plan payload is missing a sheets object');
  }

  const sheets: Partial<Record<SheetKind, SheetUsage>> = {};
  for (const kind of SHEET_KINDS) {
    const entry = rawSheets[kind];
    if (entry !== undefined) {
      sheets[kind] = decodeSheetUsage(entry, kind);
    }
  }

  return {
    plan: requireString(data, 'plan'),
    sheets,
    consentAt: decodeConsentAt(data),
    consentVersion: optionalString(data, 'llm_consent_version') ?? null,
    trialRemaining: decodeTrialRemaining(data),
    llmConsentRequired: decodeFlag(data, 'llm_consent_required'),
    lastFreeGeneration: decodeFlag(data, 'last_free_generation'),
    activeGenerationId: optionalString(data, 'active_generation_id') ?? null,
    billing: decodeBilling(data),
    ...decodeOwnedCampaigns(data),
  };
}

function decodeOwnedCampaigns(data: Record<string, unknown>): { ownedCampaigns?: SheetUsage } {
  const owned = data['owned_campaigns'];
  return owned === undefined ? {} : { ownedCampaigns: decodeSheetUsage(owned, 'owned_campaigns') };
}

function decodeBilling(data: Record<string, unknown>): BillingStatus {
  return {
    subscriptionStatus: decodeEnum(data, 'subscription_status', SUBSCRIPTION_STATUSES),
    currentPeriodEnd: decodePeriodEnd(data),
    cancelAtPeriodEnd: decodeFlag(data, 'cancel_at_period_end'),
    paymentIssue: decodeFlag(data, 'payment_issue'),
    refundEligible: decodeFlag(data, 'refund_eligible'),
    refundState: decodeEnum(data, 'refund_state', REFUND_STATES),
  };
}

function decodeEnum<const T extends string>(
  data: Record<string, unknown>,
  key: string,
  allowed: readonly T[],
): T | null {
  const value = data[key];
  if (value === undefined || value === null) {
    return null;
  }
  if (typeof value !== 'string' || !(allowed as readonly string[]).includes(value)) {
    throw drift(`plan field "${key}" is not a known value`);
  }
  return value as T;
}

function decodePeriodEnd(data: Record<string, unknown>): string | null {
  const end = optionalString(data, 'current_period_end');
  if (end === undefined) {
    return null;
  }
  if (Number.isNaN(Date.parse(end))) {
    throw drift('plan period end is not a timestamp');
  }
  return end;
}

function decodeTrialRemaining(data: Record<string, unknown>): number | undefined {
  const trial = data['trial'];
  if (trial === undefined || trial === null) {
    return undefined;
  }
  if (!isPlainObject(trial)) {
    throw drift('plan trial is not an object');
  }
  return requireNumber(trial, 'remaining');
}

function decodeFlag(data: Record<string, unknown>, key: string): boolean {
  const value = data[key];
  if (value === undefined || value === null) {
    return false;
  }
  if (typeof value !== 'boolean') {
    throw drift(`plan flag "${key}" is not a boolean`);
  }
  return value;
}

function decodeConsentAt(data: Record<string, unknown>): string | null {
  const at = optionalString(data, 'llm_consent_at');
  if (at === undefined) {
    return null;
  }
  if (Number.isNaN(Date.parse(at))) {
    throw drift('plan consent date is not a timestamp');
  }
  return at;
}

export function isSheetLimitReached(usage: PlanUsage | undefined, kind: SheetKind): boolean {
  const entry = usage?.sheets[kind];
  return entry !== undefined && entry.used >= entry.limit;
}
