/**
 * The single envelope decoder (API-02, API-03, API-04, API-22).
 *
 * Every operation returns its payload through here, so the contract lives in one
 * place: a 2xx must carry the data key, a failure must carry a non-empty error
 * code, and anything else is a defensive parse failure rather than a silent
 * success or a misclassified failure.
 */

import { DefensiveParseFailure, DomainFailure } from './errors';
import { isPlainObject } from './parse';

/** Input of {@link decodeEnvelope}. */
export interface EnvelopeInput<T> {
  /** HTTP status of the response. */
  readonly status: number;
  /** Raw response text. */
  readonly body: string;
  /** Decodes the `data` value of a successful envelope into the operation's type. */
  readonly decode: (data: unknown) => T;
  /** Raw `Retry-After` header value, carried onto a domain failure as a wait hint. */
  readonly retryAfter?: string;
}

/**
 * Parses a `Retry-After` header into milliseconds.
 *
 * The backend only sends whole seconds; anything else (an HTTP date, a negative
 * or fractional value, junk) yields no hint rather than a guessed one.
 */
export function parseRetryAfterMs(value: string | undefined): number | undefined {
  if (value === undefined || !/^\d+$/.test(value.trim())) {
    return undefined;
  }

  return Number(value.trim()) * 1000;
}

function parseBody(body: string, status: number): unknown {
  if (body === '') {
    throw new DefensiveParseFailure({ reason: 'response body is empty', status });
  }

  try {
    return JSON.parse(body);
  } catch {
    throw new DefensiveParseFailure({ reason: 'response body is not valid JSON', status });
  }
}

function decodeSuccess<T>(parsed: unknown, input: EnvelopeInput<T>): T {
  if (!isPlainObject(parsed) || !('data' in parsed)) {
    throw new DefensiveParseFailure({
      reason: 'success response is missing the data envelope',
      status: input.status,
    });
  }

  return input.decode(parsed['data']);
}

function decodeFailure(parsed: unknown, status: number, retryAfter: string | undefined): never {
  const errorBody = isPlainObject(parsed) ? parsed['error'] : undefined;
  const code = isPlainObject(errorBody) ? errorBody['code'] : undefined;

  if (typeof code !== 'string' || code === '') {
    throw new DefensiveParseFailure({
      reason: 'error response is missing error.code',
      status,
    });
  }

  const message = isPlainObject(errorBody) ? errorBody['message'] : undefined;

  throw new DomainFailure({
    kind: 'unknown',
    status,
    code,
    message: typeof message === 'string' ? message : '',
    retryAfterMs: parseRetryAfterMs(retryAfter),
  });
}

/**
 * Decodes one envelope response, handing the payload to [input].decode.
 *
 * The failure branch never calls `decode`: a refused call has no payload, and
 * returning one would hide the refusal.
 */
export function decodeEnvelope<T>(input: EnvelopeInput<T>): T {
  const parsed = parseBody(input.body, input.status);

  if (input.status >= 200 && input.status < 300) {
    return decodeSuccess(parsed, input);
  }

  return decodeFailure(parsed, input.status, input.retryAfter);
}
