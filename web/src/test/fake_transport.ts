import type { HttpRequest, HttpResponse, Transport } from '../api/transport';

export type FakeOutcome = HttpResponse | { failure: unknown };

export interface FakeTransport {
  readonly requests: HttpRequest[];
  readonly transport: Transport;
  readonly attempts: number;
  readonly lastRequest: HttpRequest | undefined;
}

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

export function successOutcome(data: unknown, status = 200): HttpResponse {
  return { status, body: JSON.stringify({ data }) };
}

export function errorOutcome(code: string, message: string, status: number): HttpResponse {
  return { status, body: JSON.stringify({ error: { code, message } }) };
}

export interface DeferredTransport {
  readonly transport: Transport;
  readonly requests: HttpRequest[];
  readonly attempts: number;
  resolve(response: HttpResponse): void;
}

export function createDeferredTransport(): DeferredTransport {
  const requests: HttpRequest[] = [];
  const pending: Array<(response: HttpResponse) => void> = [];

  const transport: Transport = (request) => {
    requests.push(request);
    return new Promise<HttpResponse>((resolve) => {
      pending.push(resolve);
    });
  };

  return {
    transport,
    requests,
    get attempts() {
      return requests.length;
    },
    resolve(response) {
      pending.shift()?.(response);
    },
  };
}

export const FREE_PLAN_USAGE = {
  plan: 'free',
  sheets: {
    character: { used: 0, limit: 1 },
    npc: { used: 0, limit: 2 },
    enemy: { used: 0, limit: 2 },
  },
};

export function withPlanUsage(
  inner: Transport,
  plan: HttpResponse = successOutcome(FREE_PLAN_USAGE),
): Transport {
  return (request) => {
    if (request.url.endsWith('/me/plan')) {
      return Promise.resolve(plan);
    }
    return inner(request);
  };
}
