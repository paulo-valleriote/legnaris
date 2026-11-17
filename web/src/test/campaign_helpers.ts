import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { errorOutcome, successOutcome } from './fake_transport';

const here = path.dirname(fileURLToPath(import.meta.url));

export const CAMPAIGN_ID = '0190c5a0-0000-7000-8000-00000000c001';
export const SHEET_ID = '0190c5a0-0000-7000-8000-00000000c002';
export const OTHER_SHEET_ID = '0190c5a0-0000-7000-8000-00000000c003';
export const MEMBER_ID = '0190c5a0-0000-7000-8000-00000000c004';
export const PLAYER_ID = '0190c5a0-0000-7000-8000-00000000c005';

export const TEMPLATE_ID = 'fantasy_v1';

export function capturedTemplates(): unknown[] {
  const raw = readFileSync(path.resolve(here, '../api/fixtures/templates.json'), 'utf8');
  const capture = JSON.parse(raw) as { response: { body: { data: unknown[] } } };
  return capture.response.body.data;
}

export function capturedSheet(): Record<string, unknown> {
  const raw = readFileSync(path.resolve(here, '../api/fixtures/sheet_get.json'), 'utf8');
  const capture = JSON.parse(raw) as { response: { body: { data: Record<string, unknown> } } };
  return capture.response.body.data;
}

export function campaignView(overrides: Record<string, unknown> = {}) {
  return {
    id: CAMPAIGN_ID,
    name: 'Ember Coast',
    template_id: TEMPLATE_ID,
    status: 'active',
    version: 3,
    gm_count: 1,
    player_count: 1,
    viewer_role: 'gm',
    viewer_owner: true,
    members: [
      {
        user_id: MEMBER_ID,
        username: 'mara',
        role: 'gm',
        owner: true,
        joined_at: '2026-11-17T10:00:00Z',
        character_name: '',
      },
      {
        user_id: PLAYER_ID,
        username: 'tomas',
        role: 'player',
        owner: false,
        joined_at: '2026-11-17T10:05:00Z',
        character_name: 'Brenna',
      },
    ],
    created_at: '2026-11-17T10:00:00Z',
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

export function campaignSummary(overrides: Record<string, unknown> = {}) {
  return {
    id: CAMPAIGN_ID,
    name: 'Ember Coast',
    template_id: TEMPLATE_ID,
    version: 3,
    viewer_role: 'gm',
    viewer_owner: true,
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

export function planWith(owned?: { used: number; limit: number }) {
  return {
    plan: 'free',
    sheets: { character: { used: 0, limit: 1 } },
    billing: { subscription_status: null, refund_eligible: false, refund_state: null },
    ...(owned === undefined ? {} : { owned_campaigns: owned }),
  };
}

export interface Exchange {
  readonly method: string;
  readonly path: string;
  readonly request: HttpRequest;
}

export type Responder = (request: HttpRequest) => HttpResponse | Promise<HttpResponse>;

export interface Route {
  readonly method: string;
  readonly path: string;
  readonly respond: Responder;
}

export interface Scripted {
  readonly transport: Transport;
  readonly exchanges: Exchange[];
  readonly calls: (method: string, path: string) => Exchange[];
}

export function scripted(routes: readonly Route[]): Scripted {
  const exchanges: Exchange[] = [];

  const transport: Transport = async (request) => {
    const url = new URL(request.url);
    const pathname = url.pathname.replace(/^\/api/, '');
    exchanges.push({ method: request.method, path: pathname, request });
    const route = routes.find(
      (candidate) => candidate.method === request.method && candidate.path === pathname,
    );
    if (route === undefined) {
      return errorOutcome('NOT_FOUND', `no route for ${request.method} ${pathname}`, 404);
    }
    return route.respond(request);
  };

  return {
    transport,
    exchanges,
    calls: (method, p) => exchanges.filter((e) => e.method === method && e.path === p),
  };
}

export function baseRoutes(
  owned: { used: number; limit: number } | null = { used: 1, limit: 5 },
): Route[] {
  return [
    {
      method: 'GET',
      path: '/me/plan',
      respond: () => successOutcome(planWith(owned ?? undefined)),
    },
    { method: 'GET', path: '/templates', respond: () => successOutcome(capturedTemplates()) },
  ];
}

export function clientFor(transport: Transport) {
  return createApiClient({
    baseUrl: 'http://api.test',
    transport,
    retryDelayMs: 0,
    sleep: () => Promise.resolve(),
  });
}

export function ok(data: unknown, status = 200): HttpResponse {
  return successOutcome(data, status);
}

export function fail(code: string, status: number): HttpResponse {
  return errorOutcome(code, code.toLowerCase(), status);
}
