import type { MessageKey } from '../l10n/strings';
import { message } from '../l10n/strings';
import type { FailureKind } from './errors';
import { DefensiveParseFailure, DomainFailure } from './errors';

const KIND_BY_CODE: Readonly<Record<string, FailureKind>> = {
  INVALID_JSON: 'clientFault',
  INVALID_REQUEST: 'invalidEdit',
  UNKNOWN_FIELD: 'renderMismatch',
  NOT_FOUND: 'notFound',
  INTERNAL_ERROR: 'serverError',
  UNAUTHORIZED: 'unauthorized',
  FORBIDDEN: 'forbidden',
  RATE_LIMITED: 'rateLimited',
  OTP_INVALID: 'invalidCode',
  EMAIL_UNAVAILABLE: 'emailUnavailable',
  CONFLICT: 'conflict',
  DISPOSABLE_EMAIL: 'disposableEmail',
  SHEET_LIMIT_REACHED: 'sheetLimitReached',
  GENERATION_IN_PROGRESS: 'generationInProgress',
  PLAN_UPGRADE_REQUIRED: 'planUpgradeRequired',
  QUOTA_EXCEEDED: 'quotaExceeded',
  LLM_CONSENT_REQUIRED: 'llmConsentRequired',
  PROVIDER_UNAVAILABLE: 'providerUnavailable',
  GENERATION_CAP_REACHED: 'generationCapReached',
};

const KIND_BY_GENERATION_ERROR: Readonly<Record<string, FailureKind>> = {
  provider_unavailable: 'providerUnavailable',
  provider_failed: 'generationFailed',
  invalid_output: 'generationFailed',
  template_unavailable: 'generationFailed',
  lease_expired: 'generationFailed',
  publish_failed: 'generationFailed',
};

const KEY_BY_KIND: Readonly<Record<FailureKind, MessageKey>> = {
  clientFault: 'api.error.clientFault',
  invalidEdit: 'api.error.invalidEdit',
  renderMismatch: 'api.error.renderMismatch',
  notFound: 'api.error.notFound',
  serverError: 'api.error.serverError',
  offline: 'api.error.offline',
  unauthorized: 'api.error.unauthorized',
  forbidden: 'api.error.forbidden',
  rateLimited: 'api.error.rateLimited',
  unknown: 'api.error.unknown',
  invalidCode: 'api.error.invalidCode',
  emailUnavailable: 'api.error.emailUnavailable',
  conflict: 'api.error.conflict',
  disposableEmail: 'api.error.disposableEmail',
  sheetLimitReached: 'api.error.sheetLimitReached',
  generationInProgress: 'api.error.generationInProgress',
  planUpgradeRequired: 'api.error.planUpgradeRequired',
  quotaExceeded: 'api.error.quotaExceeded',
  llmConsentRequired: 'api.error.llmConsentRequired',
  providerUnavailable: 'api.error.providerUnavailable',
  generationCapReached: 'api.error.generationCapReached',
  generationFailed: 'api.error.generationFailed',
  generationTimeout: 'api.error.generationTimeout',
};

export function apperrKind(code: string): FailureKind {
  return KIND_BY_CODE[code] ?? 'unknown';
}

export function generationErrorKind(errorCode: string | undefined): FailureKind {
  if (errorCode === undefined) {
    return 'generationFailed';
  }
  return KIND_BY_GENERATION_ERROR[errorCode] ?? 'generationFailed';
}

export function failureMessage(kind: FailureKind): string {
  return message(KEY_BY_KIND[kind]);
}

export function failureKind(error: unknown): FailureKind {
  return error instanceof DomainFailure ? error.kind : 'unknown';
}

export function parseFailureMessage(): string {
  return message('api.error.parse');
}

export function failureCopy(error: unknown): string {
  if (error instanceof DefensiveParseFailure) {
    return parseFailureMessage();
  }
  return failureMessage(error instanceof DomainFailure ? error.kind : 'unknown');
}

export function classifyDomainFailure(failure: DomainFailure): DomainFailure {
  return new DomainFailure({
    kind: apperrKind(failure.code ?? ''),
    status: failure.status,
    code: failure.code,
    message: failure.message,
    retryAfterMs: failure.retryAfterMs,
  });
}

export function offlineFailure(): DomainFailure {
  return new DomainFailure({ kind: 'offline' });
}
