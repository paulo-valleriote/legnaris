/**
 * The client's failure taxonomy (API-03, API-04, API-14, API-24).
 *
 * Three distinct shapes, because the interface must tell three different things
 * apart: the server refused us (domain failure), we cannot read the server
 * (defensive parse failure), and the caller addressed a value with a malformed
 * path (path failure).
 */

/**
 * UI-facing class of a failure; a screen branches on this instead of parsing a
 * response (API-15 … API-21).
 */
export type FailureKind =
  | 'clientFault'
  | 'invalidEdit'
  | 'renderMismatch'
  | 'notFound'
  | 'serverError'
  | 'offline'
  | 'unknown';

/** Input of a {@link DomainFailure}. */
export interface DomainFailureInput {
  /** The interface-facing classification (API-15 … API-21). */
  kind: FailureKind;
  /** HTTP status, when a response was received. */
  status?: unknown;
  /** Raw backend code (e.g. `NOT_FOUND`), when the envelope carried one. */
  code?: unknown;
  /** Server message, when the envelope carried a string one. */
  message?: unknown;
}

/**
 * A failed call the server answered, or a transport failure with its attempts
 * exhausted (API-03, API-20, API-24).
 *
 * Carries the status, the raw code and the server message so later logging never
 * repeats the call. Non-string values from a drifted payload are dropped rather
 * than coerced: a coerced `code` would classify wrongly and hide the drift.
 */
export class DomainFailure extends Error {
  readonly kind: FailureKind;
  readonly status: number | undefined;
  readonly code: string | undefined;

  constructor(input: DomainFailureInput) {
    super(typeof input.message === 'string' ? input.message : '');
    this.name = 'DomainFailure';
    this.kind = input.kind;
    this.status = typeof input.status === 'number' ? input.status : undefined;
    this.code = typeof input.code === 'string' ? input.code : undefined;
  }
}

/** Input of a {@link DefensiveParseFailure}. */
export interface DefensiveParseFailureInput {
  /** What failed to match — an empty body, invalid JSON, a missing key, a drifted field. */
  reason: string;
  /** HTTP status, when a response was received. */
  status?: number;
}

/**
 * A response that does not match the envelope or the expected model shape
 * (API-04, API-22, API-35).
 *
 * Deliberately not a {@link DomainFailure}: contract drift must stay
 * distinguishable from a refusal the server actually sent.
 */
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

/**
 * A malformed dotted path — empty, or carrying an empty segment (API-14).
 *
 * Raised only for a malformed path; a path that points at nothing yields no
 * value instead, because absent data is legitimate and a malformed path is a bug.
 */
export class PathFailure extends Error {
  readonly path: string;

  constructor(path: string) {
    super(`malformed data path: "${path}"`);
    this.name = 'PathFailure';
    this.path = path;
  }
}
