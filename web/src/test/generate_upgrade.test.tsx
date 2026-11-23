import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { renderApp } from './render';
import { TEMPLATE_ID, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const CATALOG = [
  {
    id: TEMPLATE_ID,
    name: 'Fantasy',
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  },
];
const GENERATION_ID = 'gen-1';
const VALID_CONCEPT = 'A wandering ranger of the north';
const UPGRADE_TITLE = 'AI generation is part of Pro';

const FREE_PLAN = {
  plan: 'free',
  sheets: {
    character: { used: 0, limit: 1 },
    npc: { used: 0, limit: 2 },
    enemy: { used: 0, limit: 2 },
  },
  owned_campaigns: { used: 0, limit: 1 },
};

const PAID_PLAN = {
  plan: 'paid',
  sheets: {
    character: { used: 0, limit: 10 },
    npc: { used: 0, limit: 50 },
    enemy: { used: 0, limit: 50 },
  },
  owned_campaigns: { used: 0, limit: 5 },
};

type Responder = HttpResponse | 'pending' | ((request: HttpRequest) => HttpResponse);

function ok(data: unknown, status = 200): HttpResponse {
  return { status, body: JSON.stringify({ data }) };
}

function fail(code: string, status: number): HttpResponse {
  return { status, body: JSON.stringify({ error: { code, message: code } }) };
}

function fakeTransport(routes: Record<string, Responder>) {
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    const key = `${request.method} ${new URL(request.url).pathname}`;
    const responder = routes[key];
    if (responder === undefined) {
      return Promise.reject(new Error(`unscripted ${key}`));
    }
    if (responder === 'pending') {
      return new Promise<HttpResponse>(() => undefined);
    }
    return Promise.resolve(typeof responder === 'function' ? responder(request) : responder);
  };
  return { requests, transport };
}

function countOf(requests: readonly HttpRequest[], method: string, path: string): number {
  return requests.filter(
    (request) => request.method === method && new URL(request.url).pathname === path,
  ).length;
}

function renderWith(routes: Record<string, Responder>) {
  const fake = fakeTransport({
    'GET /templates': ok(CATALOG),
    [`GET /templates/${TEMPLATE_ID}`]: ok(templateDetailPayload()),
    ...routes,
  });
  const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderApp('/', { client, queryClient });
  return fake;
}

function renderGenerateWith(routes: Record<string, Responder>) {
  const fake = fakeTransport({
    [`GET /templates/${TEMPLATE_ID}`]: ok(templateDetailPayload()),
    ...routes,
  });
  const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderApp(`/generate/${TEMPLATE_ID}`, { client, queryClient });
  return fake;
}

async function openCreateDialog(routes: Record<string, Responder>) {
  const user = userEvent.setup();
  const fake = renderWith(routes);
  await user.click(await screen.findByRole('button', { name: /Fantasy/ }));
  await screen.findByTestId('detail-name');
  return { user, fake };
}

function entryButton() {
  return screen.getByRole('button', { name: 'Generate with AI' });
}

function upgradeDialog() {
  return screen.getByRole('dialog', { name: UPGRADE_TITLE });
}

describe('the AI generation entry on Free (LGN-343)', () => {
  it('Happy path: the entry stays visible with a Pro badge and opens the upgrade dialog', async () => {
    const { user, fake } = await openCreateDialog({ 'GET /me/plan': ok(FREE_PLAN) });

    const entry = entryButton();
    expect(within(entry).getByText('Pro')).toBeInTheDocument();
    await user.click(entry);

    expect(upgradeDialog()).toBeInTheDocument();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);
  });

  it('Happy path: the dialog highlights AI generation and lists the paid limits', async () => {
    const { user } = await openCreateDialog({ 'GET /me/plan': ok(FREE_PLAN) });

    await user.click(entryButton());

    const dialog = upgradeDialog();
    expect(within(dialog).getByText('AI generation')).toBeInTheDocument();
    expect(within(dialog).getByText('30 AI generations per UTC day')).toBeInTheDocument();
    expect(
      within(dialog).getByText('10 character, 50 NPC and 50 enemy sheets'),
    ).toBeInTheDocument();
    expect(within(dialog).getByText('5 owned campaigns')).toBeInTheDocument();
    expect(within(dialog).getByText('No ads')).toBeInTheDocument();
  });

  it('Happy path: Not now closes the dialog and keeps the creation surface open', async () => {
    const { user } = await openCreateDialog({ 'GET /me/plan': ok(FREE_PLAN) });
    await user.click(entryButton());

    await user.click(screen.getByRole('button', { name: 'Not now' }));

    expect(screen.queryByRole('dialog', { name: UPGRADE_TITLE })).toBeNull();
    expect(screen.getByRole('dialog', { name: 'New sheet' })).toBeInTheDocument();
  });

  it('Happy path: See plans navigates to the plans page', async () => {
    const { user } = await openCreateDialog({ 'GET /me/plan': ok(FREE_PLAN) });
    await user.click(entryButton());

    await user.click(screen.getByRole('link', { name: 'See plans' }));

    expect(await screen.findByRole('heading', { name: 'Plans and billing' })).toBeInTheDocument();
  });
});

describe('the upgrade fallback from the generation API (LGN-343)', () => {
  it('Happy path: a paid plan has no badge and generates normally', async () => {
    const { user, fake } = await openCreateDialog({
      'GET /me/plan': ok(PAID_PLAN),
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'succeeded',
        kind: 'character',
        template_id: TEMPLATE_ID,
        values: { stats: { strength: 12 } },
        created_at: '2026-11-14T10:00:00Z',
      }),
    });

    const entry = screen.getByRole('link', { name: 'Generate with AI' });
    expect(screen.queryByText('Pro')).toBeNull();
    await user.click(entry);
    await screen.findByLabelText('Describe your character');
    fireEvent.change(screen.getByLabelText('Describe your character'), {
      target: { value: VALID_CONCEPT },
    });
    await user.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => expect(countOf(fake.requests, 'POST', '/generations')).toBe(1));
  });

  it('Error: PLAN_UPGRADE_REQUIRED opens the upgrade dialog and no inline alert', async () => {
    const user = userEvent.setup();
    renderGenerateWith({
      'GET /me/plan': ok(PAID_PLAN),
      'POST /generations': fail('PLAN_UPGRADE_REQUIRED', 403),
    });
    await screen.findByLabelText('Describe your character');
    fireEvent.change(screen.getByLabelText('Describe your character'), {
      target: { value: VALID_CONCEPT },
    });

    await user.click(screen.getByRole('button', { name: 'Generate' }));

    expect(await screen.findByRole('dialog', { name: UPGRADE_TITLE })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('the generation gate on Free (LGN-343)', () => {
  it('Edge: Free never sees the consent prompt and sends no request', async () => {
    const fake = renderGenerateWith({
      'GET /me/plan': ok({
        ...FREE_PLAN,
        llm_consent_required: true,
      }),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(screen.getByLabelText('Describe your character'), {
      target: { value: VALID_CONCEPT },
    });

    await user.click(screen.getByRole('button', { name: 'Generate' }));

    expect(await screen.findByRole('dialog', { name: UPGRADE_TITLE })).toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: 'Send your concept to an AI model?' })).toBeNull();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);
    expect(countOf(fake.requests, 'PUT', '/me/llm-consent')).toBe(0);
  });
});

describe('the AI generation entry while the plan is unresolved (LGN-343)', () => {
  it('Edge: a plan still loading keeps the entry a link with no badge', async () => {
    await openCreateDialog({ 'GET /me/plan': 'pending' });

    expect(screen.getByRole('link', { name: 'Generate with AI' })).toBeInTheDocument();
    expect(screen.queryByText('Pro')).toBeNull();
  });

  it('Edge: a plan fetch error keeps the entry a link with no badge', async () => {
    await openCreateDialog({ 'GET /me/plan': fail('INTERNAL_ERROR', 500) });

    expect(screen.getByRole('link', { name: 'Generate with AI' })).toBeInTheDocument();
    expect(screen.queryByText('Pro')).toBeNull();
  });
});
