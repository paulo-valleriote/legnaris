import { screen } from '@testing-library/react';
import { QueryClient } from '@tanstack/react-query';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { renderApp } from './render';
import { TEMPLATE_ID, templateDetailPayload } from './sheet_fixtures';

export const BASE_URL = 'http://api.test';
export const GENERATION_ID = 'gen-1';
export const ROUTE = `/generate/${TEMPLATE_ID}`;
export const VALID_CONCEPT = 'A wandering ranger of the north';

export type Responder = HttpResponse | ((request: HttpRequest) => HttpResponse);

export function ok(data: unknown, status = 200): HttpResponse {
  return { status, body: JSON.stringify({ data }) };
}

export function fail(code: string, status: number, retryAfter?: string): HttpResponse {
  return {
    status,
    body: JSON.stringify({ error: { code, message: code } }),
    ...(retryAfter === undefined ? {} : { retryAfter }),
  };
}

export function planWith(overrides: Record<string, unknown> = {}) {
  return {
    plan: 'paid',
    sheets: { character: { used: 0, limit: 10 } },
    llm_consent_required: false,
    active_generation_id: null,
    ...overrides,
  };
}

export function routed(routes: Record<string, Responder>) {
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    const key = `${request.method} ${new URL(request.url).pathname}`;
    const entry = routes[key];
    if (entry === undefined) {
      return Promise.reject(new Error(`unscripted ${key}`));
    }
    return Promise.resolve(typeof entry === 'function' ? entry(request) : entry);
  };
  return { requests, transport };
}

export function countOf(requests: readonly HttpRequest[], method: string, path: string): number {
  return requests.filter(
    (request) => request.method === method && new URL(request.url).pathname === path,
  ).length;
}

export function renderGenerate(routes: Record<string, Responder>, state?: unknown) {
  const fake = routed({
    [`GET /templates/${TEMPLATE_ID}`]: ok(templateDetailPayload()),
    'GET /me/plan': ok(planWith()),
    ...routes,
  });
  const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderApp(ROUTE, { client, queryClient, ...(state === undefined ? {} : { routeState: state }) });
  return fake;
}

export function conceptField() {
  return screen.getByLabelText('Describe your character');
}

export function generateButton() {
  return screen.getByRole('button', { name: 'Generate' });
}
