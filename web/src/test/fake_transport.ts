import type { HttpRequest, HttpResponse, Transport } from '../api/transport';

/** One scripted outcome for a single transport attempt. */
export type FakeOutcome = HttpResponse | { failure: unknown };

/** A transport that records every request and replays scripted outcomes. */
export interface FakeTransport {
  /** Every request received, in order. */
  readonly requests: HttpRequest[];
  /** The transport function to inject into the client. */
  readonly transport: Transport;
  /** Number of attempts made so far. */
  readonly attempts: number;
  /** The most recent request, or `undefined` when none was made. */
  readonly lastRequest: HttpRequest | undefined;
}

/**
 * Builds a recording transport for the API client suites.
 *
 * Outcomes are consumed in order; the last one repeats, so a test scripts "the
 * read fails once, then succeeds" without knowing the retry count in advance.
 * An outcome without `status` is a transport-level failure and rejects, which is
 * what the retry rules branch on.
 */
export function createFakeTransport(...outcomes: FakeOutcome[]): FakeTransport {
  if (outcomes.length === 0) {
    throw new Error('createFakeTransport needs at least one outcome');
  }

  const requests: HttpRequest[] = [];

  const transport: Transport = (request) => {
    requests.push(request);

    const outcome = outcomes[Math.min(requests.length - 1, outcomes.length - 1)];

    if (outcome && 'failure' in outcome) {
      return Promise.reject(outcome.failure);
    }

    return Promise.resolve(outcome as HttpResponse);
  };

  return {
    requests,
    transport,
    get attempts() {
      return requests.length;
    },
    get lastRequest() {
      return requests.at(-1);
    },
  };
}

/** Wraps a body in a successful envelope response, as the backend emits it. */
export function successOutcome(data: unknown, status = 200): HttpResponse {
  return { status, body: JSON.stringify({ data }) };
}

/** Wraps a code and message in a failure envelope response (API-03). */
export function errorOutcome(code: string, message: string, status: number): HttpResponse {
  return { status, body: JSON.stringify({ error: { code, message } }) };
}
