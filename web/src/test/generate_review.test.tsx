import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PREVIEW_THROTTLE_MS } from '../generate/generation_flow';
import {
  BASE_URL,
  GENERATION_ID,
  VALID_CONCEPT,
  conceptField,
  countOf,
  fail,
  generateButton,
  ok,
  planWith,
  renderGenerate,
  routed,
} from './generate_fixtures';
import { renderApp } from './render';
import { TEMPLATE_ID, templateDetailPayload } from './sheet_fixtures';
import { createApiClient } from '../api/client';
import { QueryClient } from '@tanstack/react-query';

const PREVIEW_PATH = `/templates/${TEMPLATE_ID}/preview`;
const SUCCEEDED = {
  id: GENERATION_ID,
  status: 'succeeded',
  kind: 'character',
  template_id: TEMPLATE_ID,
  values: { identity: { characterName: 'Aragorn', level: 3 } },
  created_at: '2026-11-14T10:00:00Z',
};

function succeededRoutes(
  extra: Record<string, ReturnType<typeof ok> | ReturnType<typeof fail>> = {},
) {
  return {
    'POST /generations': ok({ id: GENERATION_ID }, 202),
    [`GET /generations/${GENERATION_ID}`]: ok(SUCCEEDED),
    ...extra,
  };
}

/** Submits the concept and waits until the review step is on screen. */
async function reachReview(
  routes: Record<string, ReturnType<typeof ok> | ReturnType<typeof fail>> = {},
  plan: Record<string, unknown> = {},
) {
  const user = userEvent.setup();
  const fake = renderGenerate({ ...succeededRoutes(routes), 'GET /me/plan': ok(planWith(plan)) });
  await screen.findByLabelText('Describe your character');
  fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
  await user.click(generateButton());
  await screen.findByRole('heading', { name: 'Review your character' });
  return { user, fake };
}

describe('review step name prefill (LGN-196)', () => {
  it('Happy path: the Sheet name is prefilled from the generated character name', async () => {
    await reachReview();

    expect(screen.getByLabelText('Sheet name')).toHaveValue('Aragorn');
  });

  it('Edge: a generated name that is blank leaves the Sheet name empty and required', async () => {
    await reachReview({
      [`GET /generations/${GENERATION_ID}`]: ok({
        ...SUCCEEDED,
        values: { identity: { characterName: '   ', level: 3 } },
      }),
    });

    expect(screen.getByLabelText('Sheet name')).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Save to my sheets' })).toBeDisabled();
  });

  it('Edge: a typed name is kept and not overwritten by the generated one', async () => {
    const { user } = await reachReview();
    const name = screen.getByLabelText('Sheet name');

    await user.clear(name);
    await user.type(name, 'Strider');

    expect(name).toHaveValue('Strider');
  });
});

describe('review step derived values (LGN-196, throttled preview)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  async function flush() {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
  }

  it('Happy path: an edit sends the changed delta to the template preview', async () => {
    const fake = renderGenerate(
      succeededRoutes({ [`POST ${PREVIEW_PATH}`]: ok({ identity: { level: 4 } }) }),
    );
    await flush();
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
    fireEvent.click(generateButton());
    await flush();

    fireEvent.change(screen.getByRole('textbox', { name: 'Level' }), { target: { value: '4' } });
    await flush();

    expect(countOf(fake.requests, 'POST', PREVIEW_PATH)).toBe(1);
    const sent = fake.requests.find((request) => request.url.endsWith(PREVIEW_PATH));
    expect(JSON.parse(sent?.body ?? '{}')).toEqual({ data: { identity: { level: 4 } } });
  });

  it('Edge: two quick edits send one preview at once and the second only after the window', async () => {
    const fake = renderGenerate(succeededRoutes({ [`POST ${PREVIEW_PATH}`]: ok({}) }));
    await flush();
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
    fireEvent.click(generateButton());
    await flush();
    const level = screen.getByRole('textbox', { name: 'Level' });

    fireEvent.change(level, { target: { value: '4' } });
    await flush();
    fireEvent.change(level, { target: { value: '5' } });
    await flush();
    expect(countOf(fake.requests, 'POST', PREVIEW_PATH)).toBe(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(PREVIEW_THROTTLE_MS);
    });
    expect(countOf(fake.requests, 'POST', PREVIEW_PATH)).toBe(2);
  });

  it('Edge: a failed preview keeps the edited value and says the figures could not update', async () => {
    renderGenerate(succeededRoutes({ [`POST ${PREVIEW_PATH}`]: fail('INTERNAL_ERROR', 500) }));
    await flush();
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
    fireEvent.click(generateButton());
    await flush();

    fireEvent.change(screen.getByRole('textbox', { name: 'Level' }), { target: { value: '4' } });
    await flush();

    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('4');
    expect(
      screen.getByText('The figures could not be recalculated. Your edits are kept.'),
    ).toBeInTheDocument();
  });
});

describe('regenerate and the last generation (LGN-196)', () => {
  it('Edge: with several left, regenerate goes straight back to the concept', async () => {
    const { user } = await reachReview({}, { trial: { used: 1, remaining: 2 } });

    await user.click(screen.getByRole('button', { name: 'Regenerate (2 left)' }));

    expect(await screen.findByLabelText('Describe your character')).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('Edge: on the last generation, regenerate asks first and keeps the review until confirmed', async () => {
    const { user } = await reachReview({}, { trial: { used: 2, remaining: 1 } });

    await user.click(screen.getByRole('button', { name: 'Regenerate (1 left)' }));

    expect(
      await screen.findByRole('dialog', { name: /last free generation/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Review your character' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Keep this character' }));
    expect(screen.getByRole('heading', { name: 'Review your character' })).toBeInTheDocument();
  });

  it('Happy path: confirming the last generation returns to the concept', async () => {
    const { user } = await reachReview({}, { trial: { used: 2, remaining: 1 } });
    await user.click(screen.getByRole('button', { name: 'Regenerate (1 left)' }));

    await user.click(await screen.findByRole('button', { name: 'Use my last one' }));

    expect(await screen.findByLabelText('Describe your character')).toBeInTheDocument();
  });

  it('Edge: once the server has spent the last generation, regenerate is disabled', async () => {
    let plansRead = 0;
    const user = userEvent.setup();
    renderGenerate({
      ...succeededRoutes(),
      'GET /me/plan': () => {
        plansRead += 1;
        return ok(
          planWith(
            plansRead === 1
              ? { trial: { used: 2, remaining: 1 } }
              : { trial: { used: 3, remaining: 0 } },
          ),
        );
      },
    });
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
    await user.click(generateButton());
    await screen.findByRole('heading', { name: 'Review your character' });

    expect(screen.getByRole('button', { name: 'Regenerate (0 left)' })).toBeDisabled();
  });
});

describe('resume from the catalog (LGN-196)', () => {
  it('Happy path: a notice handed over with the route resumes the waiting generation', async () => {
    renderGenerate(succeededRoutes(), { generationId: GENERATION_ID });

    expect(
      await screen.findByRole('heading', { name: 'Review your character' }),
    ).toBeInTheDocument();
  });

  it('Edge: the catalog notice opens the waiting generation through its template', async () => {
    const fake = routed({
      'GET /templates': ok([{ id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} }]),
      [`GET /templates/${TEMPLATE_ID}`]: ok(templateDetailPayload()),
      'GET /me/plan': ok(planWith({ active_generation_id: GENERATION_ID })),
      [`GET /generations/${GENERATION_ID}`]: ok(SUCCEEDED),
    });
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const user = userEvent.setup();
    renderApp('/', { client, queryClient });

    await user.click(await screen.findByRole('button', { name: 'Open' }));

    expect(
      await screen.findByRole('heading', { name: 'Review your character' }),
    ).toBeInTheDocument();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);
  });
});
