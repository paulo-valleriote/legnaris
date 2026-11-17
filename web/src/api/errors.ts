export type FailureKind =
  | 'clientFault'
  | 'invalidEdit'
  | 'renderMismatch'
  | 'notFound'
  | 'serverError'
  | 'offline'
  | 'unauthorized'
  | 'forbidden'
  | 'rateLimited'
  | 'unknown'
  | 'invalidCode'
  | 'emailUnavailable'
  | 'conflict'
  | 'disposableEmail'
  | 'sheetLimitReached'
  | 'generationInProgress'
  | 'planUpgradeRequired'
  | 'quotaExceeded'
  | 'llmConsentRequired'
  | 'providerUnavailable'
  | 'generationCapReached'
  | 'generationFailed'
  | 'generationTimeout'
  | 'alreadySubscribed'
  | 'billingUnavailable'
  | 'noSubscription'
  | 'refundNotEligible'
  | 'campaignNotFound'
  | 'campaignFull'
  | 'campaignForbidden'
  | 'invalidInvite'
  | 'templateMismatch'
  | 'sheetAlreadyLinked'
  | 'planLimitReached';

export interface DomainFailureInput {
  kind: FailureKind;
  status?: unknown;
  code?: unknown;
  message?: unknown;
  retryAfterMs?: unknown;
}

export class DomainFailure extends Error {
  readonly kind: FailureKind;
  readonly status: number | undefined;
  readonly code: string | undefined;
  readonly retryAfterMs: number | undefined;

  constructor(input: DomainFailureInput) {
    super(typeof input.message === 'string' ? input.message : '');
    this.name = 'DomainFailure';
    this.kind = input.kind;
    this.status = typeof input.status === 'number' ? input.status : undefined;
    this.code = typeof input.code === 'string' ? input.code : undefined;
    this.retryAfterMs =
      typeof input.retryAfterMs === 'number' && input.retryAfterMs >= 0
        ? input.retryAfterMs
        : undefined;
  }
}

export interface DefensiveParseFailureInput {
  reason: string;
  status?: number;
}

export class DefensiveParseFailure extends Error {
  readonly reason: string;
  readonly status: number | undefined;

  constructor(input: DefensiveParseFailureInput) {
    super(`unreadable response: ${input.reason}`);
    this.name = 'DefensiveParseFailure';
    this.reason = input.reason;
    this.status = input.status;
  }
}

export class PathFailure extends Error {
  readonly path: string;

  constructor(path: string) {
    super(`malformed data path: "${path}"`);
    this.name = 'PathFailure';
    this.path = path;
  }
}

export class ConfigurationFailure extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ConfigurationFailure';
  }
}
