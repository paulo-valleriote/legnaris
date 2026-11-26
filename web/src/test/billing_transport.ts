import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';

export const BILLING_BASE_URL = 'http://api.test';

export const CHECKOUT_URL = 'https://checkout.stripe.com/c/pay/cs_test_abc';
export const PORTAL_URL = 'https://billing.stripe.com/p/session/bps_test_xyz';

export const FREE_NEVER_SUBSCRIBED = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
  llm_consent_at: null,
  llm_consent_version: null,
  subscription_status: null,
  payment_issue: false,
  refund_eligible: false,
  refund_state: null,
};

export const PAID_ACTIVE = {
  ...FREE_NEVER_SUBSCRIBED,
  plan: 'paid',
  subscription_status: 'active',
  subscription_provider: 'stripe',
  current_period_end: '2026-12-16T00:00:00Z',
  cancel_at_period_end: false,
};

export type Route = HttpResponse | Promise<HttpResponse>;

export interface BillingScript {
  readonly plans: readonly Route[];
  readonly checkout?: Route;
  readonly portal?: Route;
  readonly refund?: Route;
}

export function json(data: unknown, status = 200): HttpResponse {
  return { status, body: JSON.stringify({ data }) };
}

export function problem(code: string, message: string, status: number): HttpResponse {
  return { status, body: JSON.stringify({ error: { code, message } }) };
}

export const ACCEPTED: HttpResponse = { status: 202, body: '' };

const ROUTE_KEYS: Readonly<Record<string, keyof Omit<BillingScript, 'plans'>>> = {
  'POST /billing/checkout': 'checkout',
  'POST /billing/portal': 'portal',
  'POST /billing/refund': 'refund',
};

export function scriptedBilling(script: BillingScript) {
  const requests: HttpRequest[] = [];
  let planReads = 0;

  const transport: Transport = (request) => {
    requests.push(request);
    const path = new URL(request.url).pathname;

    if (request.method === 'GET' && path === '/me/plan') {
      const outcome = script.plans[Math.min(planReads, script.plans.length - 1)];
      planReads += 1;
      return Promise.resolve(outcome as Route);
    }

    const key = ROUTE_KEYS[`${request.method} ${path}`];
    const route = key === undefined ? undefined : script[key];
    if (route === undefined) {
      return Promise.reject(new Error(`unexpected request ${request.method} ${path}`));
    }
    return Promise.resolve(route);
  };

  return {
    transport,
    client: createApiClient({ baseUrl: BILLING_BASE_URL, transport, retryDelayMs: 0 }),
    requests,
    posts: (path: string) =>
      requests.filter((request) => request.method === 'POST' && request.url.endsWith(path)),
    planReads: () => planReads,
  };
}
