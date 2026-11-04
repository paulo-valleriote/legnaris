/**
 * Failure classification and copy resolution (API-15 … API-24).
 *
 * The backend speaks in codes; the interface branches on kinds. This module is
 * the only translation point, so a screen never parses a code and the copy a
 * person sees is resolved from the translation catalog rather than typed here.
 * The three refusals the code step introduced — the refused code, the
 * unavailable send and the duplicate address — carry a kind and a copy of their
 * own, so they never collapse into the generic sentence (WCS-11).
 */

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
};

/**
 * Maps a backend code to the interface-facing kind (API-15 … API-21, WAS-30).
 *
 * The codes with a dedicated kind map to it; every other code — the fallback
 * codes the backend defines and any code it does not — takes the generic kind,
 * so an unmapped code is visible rather than misclassified.
 */
export function apperrKind(code: string): FailureKind {
  return KIND_BY_CODE[code] ?? 'unknown';
}

/** Resolves the copy for a failure kind from the translation catalog (API-23). */
export function failureMessage(kind: FailureKind): string {
  return message(KEY_BY_KIND[kind]);
}

/**
 * Resolves the interface-facing kind of a failure (WCS-27).
 *
 * A surface branches on the kind — the duplicate-address refusal carries an
 * action the person can take — while the copy comes from the catalog.
 */
export function failureKind(error: unknown): FailureKind {
  return error instanceof DomainFailure ? error.kind : 'unknown';
}

/** Resolves the copy for a response the client could not read (API-22, API-23). */
export function parseFailureMessage(): string {
  return message('api.error.parse');
}

/**
 * Resolves the copy a screen shows for a failed read or write (API-22, API-23).
 *
 * A payload the client could not decode resolves the parse copy; every other
 * failure resolves its interface-facing kind, an unclassified error included.
 * Shared by every screen so the mapping lives in one place.
 */
export function failureCopy(error: unknown): string {
  if (error instanceof DefensiveParseFailure) {
    return parseFailureMessage();
  }
  return failureMessage(error instanceof DomainFailure ? error.kind : 'unknown');
}

/**
 * Rewrites a decoded domain failure with its interface-facing kind (API-15 …
 * API-21, API-24).
 *
 * The envelope decoder cannot know the kind — it sees only the raw code — so the
 * client passes every domain failure through here; the raw code, the status and
 * the server message survive unchanged.
 */
export function classifyDomainFailure(failure: DomainFailure): DomainFailure {
  return new DomainFailure({
    kind: apperrKind(failure.code ?? ''),
    status: failure.status,
    code: failure.code,
    message: failure.message,
    retryAfterMs: failure.retryAfterMs,
  });
}

/**
 * Builds the failure for a request that failed at the transport with its
 * attempts exhausted (API-20).
 *
 * It carries no status (no response was received), no code (the backend is not
 * speaking) and no transport text, so no identifier-bearing message leaks
 * through (API-29).
 */
export function offlineFailure(): DomainFailure {
  return new DomainFailure({ kind: 'offline' });
}
