import { DefensiveParseFailure, DomainFailure } from './errors';
import { isPlainObject } from './parse';

export interface EnvelopeInput<T> {
  readonly status: number;
  readonly body: string;
  readonly decode: (data: unknown) => T;
  readonly retryAfter?: string;
}

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

export function decodeEnvelope<T>(input: EnvelopeInput<T>): T {
  const parsed = parseBody(input.body, input.status);

  if (input.status >= 200 && input.status < 300) {
    return decodeSuccess(parsed, input);
  }

  return decodeFailure(parsed, input.status, input.retryAfter);
}
