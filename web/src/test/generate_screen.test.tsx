import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { renderApp } from './render';
import { TEMPLATE_ID, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const GENERATION_ID = 'gen-1';
const ROUTE = `/generate/${TEMPLATE_ID}`;

type Responder = HttpResponse | ((request: HttpRequest) => HttpResponse);

function ok(data: unknown, status = 200): HttpResponse {
  return { status, body: JSON.stringify({ data }) };
}

function fail(code: string, status: number, retryAfter?: string): HttpResponse {
  return {
    status,
    body: JSON.stringify({ error: { code, message: code } }),
    ...(retryAfter === undefined ? {} : { retryAfter }),
  };
}

function planWith(overrides: Record<string, unknown> = {}) {
  return {
    plan: 'free',
    sheets: { character: { used: 0, limit: 1 } },
    trial: { used: 0, remaining: 3 },
    llm_consent_required: false,
    active_generation_id: null,
    ...overrides,
  };
}

/** A transport that answers by `METHOD /path` and records every request. */
function routed(routes: Record<string, Responder>) {
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

function countOf(requests: readonly HttpRequest[], method: string, path: string) {
  return requests.filter(
    (request) => request.method === method && new URL(request.url).pathname === path,
  ).length;
}

function renderGenerate(routes: Record<string, Responder>) {
  const fake = routed({
    [`GET /templates/${TEMPLATE_ID}`]: ok(templateDetailPayload()),
    'GET /me/plan': ok(planWith()),
    ...routes,
  });
  const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderApp(ROUTE, { client, queryClient });
  return fake;
}

function conceptField() {
  return screen.getByLabelText('Describe your character');
}

function generateButton() {
  return screen.getByRole('button', { name: 'Generate' });
}

const VALID_CONCEPT = 'A wandering ranger of the north';

describe('generate concept step (LGN-196, LGN-177)', () => {
  it.each([
    { name: 'Edge: 9 characters keeps Generate disabled', text: '123456789', enabled: false },
    { name: 'Edge: 10 characters enables Generate', text: '1234567890', enabled: true },
    { name: 'Edge: 1000 characters enables Generate', text: 'a'.repeat(1000), enabled: true },
    {
      name: 'Edge: 1001 characters disables Generate and says it is too long',
      text: 'a'.repeat(1001),
      enabled: false,
    },
  ])('$name', async ({ text, enabled }) => {
    renderGenerate({});
    await screen.findByLabelText('Describe your character');

    fireEvent.change(conceptField(), { target: { value: text } });

    if (enabled) {
      expect(generateButton()).toBeEnabled();
    } else {
      expect(generateButton()).toBeDisabled();
    }
  });

  it('Edge: a trial with nothing left shows none left and never sends a request', async () => {
    const fake = renderGenerate({
      'GET /me/plan': ok(planWith({ trial: { used: 3, remaining: 0 } })),
    });
    await screen.findByText('You have no free generations left.');

    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    expect(generateButton()).toBeDisabled();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);
  });

  it('Happy path: shows the trial count from the plan', async () => {
    renderGenerate({});

    expect(await screen.findByText('3 of 3 free generations left')).toBeInTheDocument();
  });
});

describe('generate flow (LGN-196)', () => {
  it('Happy path: concept, wait, review and accept opens the Sheet (201)', async () => {
    const fake = renderGenerate({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'succeeded',
        kind: 'character',
        template_id: TEMPLATE_ID,
        values: { stats: { strength: 12 } },
        created_at: '2026-11-14T10:00:00Z',
      }),
      [`POST /generations/${GENERATION_ID}/accept`]: ok({ sheet_id: 'sheet-9' }, 201),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: `  ${VALID_CONCEPT}  ` } });

    await user.click(generateButton());

    const sheetName = await screen.findByLabelText('Sheet name');
    const save = screen.getByRole('button', { name: 'Save to my sheets' });
    expect(save).toBeDisabled();
    await user.type(sheetName, 'Aragorn');
    await user.click(save);

    await waitFor(() =>
      expect(countOf(fake.requests, 'POST', `/generations/${GENERATION_ID}/accept`)).toBe(1),
    );
    const created = fake.requests.find(
      (request) => request.method === 'POST' && request.url.endsWith('/generations'),
    );
    expect(JSON.parse(created?.body ?? '{}')).toEqual({
      template_id: TEMPLATE_ID,
      kind: 'character',
      concept: VALID_CONCEPT,
    });
    const accepted = fake.requests.find((request) => request.url.endsWith('/accept'));
    expect(JSON.parse(accepted?.body ?? '{}')).toEqual({
      sheet_name: 'Aragorn',
      values: { stats: { strength: 12 } },
    });
  });

  it('Edge: a generation that failed shows its failure copy and offers to try again', async () => {
    renderGenerate({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'failed',
        kind: 'character',
        template_id: TEMPLATE_ID,
        error_code: 'invalid_output',
        created_at: '2026-11-14T10:00:00Z',
      }),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    expect(await screen.findByText(/We couldn't generate it/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('Error: PLAN_UPGRADE_REQUIRED shows the allowance copy and no upsell', async () => {
    renderGenerate({
      'POST /generations': fail('PLAN_UPGRADE_REQUIRED', 403),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'You have used your free character generations.',
    );
    expect(screen.queryByRole('button', { name: /checkout|upgrade|subscribe/i })).toBeNull();
  });

  it('Error: a quota refusal shows the reset copy', async () => {
    renderGenerate({
      'POST /generations': fail('QUOTA_EXCEEDED', 429, '60'),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    expect(await screen.findByRole('alert')).toHaveTextContent(
      "You have used today's generations.",
    );
  });

  it('Edge: GENERATION_IN_PROGRESS resumes the active generation instead of failing', async () => {
    renderGenerate({
      'POST /generations': fail('GENERATION_IN_PROGRESS', 409),
      'GET /me/plan': ok(planWith({ active_generation_id: GENERATION_ID })),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'accepted',
        kind: 'character',
        template_id: TEMPLATE_ID,
        created_at: '2026-11-14T10:00:00Z',
      }),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    expect(
      await screen.findByText('This character was already saved to your sheets.'),
    ).toBeInTheDocument();
  });

  it('Error: a failed plan refresh after GENERATION_IN_PROGRESS shows the mapped error, not an unhandled rejection', async () => {
    let planReads = 0;
    renderGenerate({
      'POST /generations': fail('GENERATION_IN_PROGRESS', 409),
      'GET /me/plan': () => {
        planReads += 1;
        return planReads === 1 ? ok(planWith()) : fail('INTERNAL_ERROR', 500);
      },
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    const alert = await screen.findByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(alert).not.toHaveTextContent('This character is still being generated');
    expect(screen.getByLabelText('Describe your character')).toBeInTheDocument();
  });

  it('Edge: a waiting generation can be left and resumed from the concept step', async () => {
    renderGenerate({
      'GET /me/plan': ok(planWith({ active_generation_id: GENERATION_ID })),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'succeeded',
        kind: 'character',
        template_id: TEMPLATE_ID,
        values: {},
        created_at: '2026-11-14T10:00:00Z',
      }),
    });
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Open' }));

    expect(
      await screen.findByRole('heading', { name: 'Review your character' }),
    ).toBeInTheDocument();
  });
});

describe('generate consent (ADR 0008, LGN-196)', () => {
  it('Happy path: consent required opens the disclosure and sends nothing until accepted', async () => {
    const fake = renderGenerate({
      'GET /me/plan': ok(planWith({ llm_consent_required: true })),
      'PUT /me/llm-consent': { status: 204, body: '' },
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'running',
        kind: 'character',
        template_id: TEMPLATE_ID,
        created_at: '2026-11-14T10:00:00Z',
      }),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    expect(
      await screen.findByRole('dialog', { name: 'Send your concept to an AI model?' }),
    ).toBeInTheDocument();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);

    await user.click(screen.getByRole('button', { name: 'I agree' }));

    await waitFor(() => expect(countOf(fake.requests, 'POST', '/generations')).toBe(1));
    const consent = fake.requests.find((request) => request.method === 'PUT');
    expect(JSON.parse(consent?.body ?? '{}')).toEqual({ version: '1' });
  });

  it('Edge: declining consent sends no request and keeps the concept', async () => {
    const fake = renderGenerate({
      'GET /me/plan': ok(planWith({ llm_consent_required: true })),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());
    await user.click(await screen.findByRole('button', { name: 'Not now' }));

    expect(countOf(fake.requests, 'PUT', '/me/llm-consent')).toBe(0);
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);
    expect(conceptField()).toHaveValue(VALID_CONCEPT);
  });

  it('Error: LLM_CONSENT_REQUIRED from the server reopens the disclosure', async () => {
    renderGenerate({
      'POST /generations': fail('LLM_CONSENT_REQUIRED', 403),
    });
    const user = userEvent.setup();
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });

    await user.click(generateButton());

    expect(
      await screen.findByRole('dialog', { name: 'Send your concept to an AI model?' }),
    ).toBeInTheDocument();
  });
});
