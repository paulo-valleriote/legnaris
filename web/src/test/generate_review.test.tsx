import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, within } from '@testing-library/react';
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
  suggested_name: 'Aragorn',
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

describe('review step name prefill (LGN-196, LGN-233)', () => {
  it('Happy path: the Sheet name is prefilled from the server suggested name', async () => {
    await reachReview();

    expect(screen.getByLabelText('Sheet name')).toHaveValue('Aragorn');
  });

  it('Edge: an absent suggested name leaves the Sheet name empty and required', async () => {
    const withoutSuggestion = { ...SUCCEEDED, suggested_name: undefined };
    await reachReview({
      [`GET /generations/${GENERATION_ID}`]: ok(JSON.parse(JSON.stringify(withoutSuggestion))),
    });

    expect(screen.getByLabelText('Sheet name')).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Save to my sheets' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
  });

  it('Edge: a null suggested name leaves the Sheet name empty, never a placeholder', async () => {
    await reachReview({
      [`GET /generations/${GENERATION_ID}`]: ok({ ...SUCCEEDED, suggested_name: null }),
    });

    expect(screen.getByLabelText('Sheet name')).toHaveValue('');
  });

  it('Edge: a blank suggested name leaves the Sheet name empty and required', async () => {
    await reachReview({
      [`GET /generations/${GENERATION_ID}`]: ok({ ...SUCCEEDED, suggested_name: '   ' }),
    });

    expect(screen.getByLabelText('Sheet name')).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Save to my sheets' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
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

describe('regenerate (LGN-196, LGN-342)', () => {
  it('Happy path: regenerate goes back to the concept step', async () => {
    const { user } = await reachReview();

    await user.click(screen.getByRole('button', { name: 'Regenerate' }));

    expect(await screen.findByLabelText('Describe your character')).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('Edge: regenerating with unsaved edits asks before discarding them', async () => {
    const { user } = await reachReview();
    fireEvent.change(screen.getByRole('textbox', { name: 'Level' }), { target: { value: '4' } });

    await user.click(screen.getByRole('button', { name: 'Regenerate' }));

    expect(
      await screen.findByRole('dialog', { name: 'Leave without saving?' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Review your character' })).toBeInTheDocument();
  });

  it('Happy path: confirming the discard returns to the concept step', async () => {
    const { user } = await reachReview();
    fireEvent.change(screen.getByRole('textbox', { name: 'Level' }), { target: { value: '4' } });
    await user.click(screen.getByRole('button', { name: 'Regenerate' }));

    await user.click(await screen.findByRole('button', { name: 'Leave' }));

    expect(await screen.findByLabelText('Describe your character')).toBeInTheDocument();
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
      'GET /templates': ok([
        {
          id: TEMPLATE_ID,
          name: 'Fantasy',
          default: true,
          kinds: ['character', 'npc', 'enemy'],
          fields: {},
        },
      ]),
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

  it('Happy path: the catalog notice names its kind and resumes an enemy generation (LGN-187)', async () => {
    const fake = routed({
      'GET /templates': ok([
        {
          id: TEMPLATE_ID,
          name: 'Fantasy',
          default: true,
          kinds: ['character', 'npc', 'enemy'],
          fields: {},
        },
      ]),
      [`GET /templates/${TEMPLATE_ID}`]: ok(templateDetailPayload()),
      'GET /me/plan': ok(planWith({ active_generation_id: GENERATION_ID })),
      [`GET /generations/${GENERATION_ID}`]: ok({ ...SUCCEEDED, kind: 'enemy' }),
    });
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const user = userEvent.setup();
    renderApp('/', { client, queryClient });

    const text = await screen.findByText('A generated enemy is waiting for you.');
    const notice = text.closest('.lg-notice');
    expect(notice).not.toBeNull();
    await user.click(within(notice as HTMLElement).getByRole('button', { name: 'Open' }));

    expect(await screen.findByRole('heading', { name: 'Review your enemy' })).toBeInTheDocument();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(0);
  });
});

const reviewKinds = [
  {
    kind: 'character',
    title: 'Review your character',
    note: "Generated values are a starting point. Check them against your game system's rules and confirm the details with your game master.",
  },
  {
    kind: 'npc',
    title: 'Review your NPC',
    note: "Generated values are a starting point. Check them against your game system's rules and adjust the details to fit your session.",
  },
  {
    kind: 'enemy',
    title: 'Review your enemy',
    note: "Generated values are a starting point. Check them against your game system's rules and adjust the details to fit your encounter.",
  },
] as const;

async function reachReviewOfKind(kind: string, title: string) {
  const user = userEvent.setup();
  renderGenerate({
    ...succeededRoutes({ [`GET /generations/${GENERATION_ID}`]: ok({ ...SUCCEEDED, kind }) }),
    'GET /me/plan': ok(planWith()),
  });
  await screen.findByLabelText('Describe your character');
  fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
  await user.click(generateButton());
  await screen.findByRole('heading', { name: title });
}

describe('review step starting-point note', () => {
  it.each(reviewKinds)(
    'Happy path: the $kind review shows its own starting-point note',
    async ({ kind, title, note }) => {
      await reachReviewOfKind(kind, title);

      expect(screen.getByText(note)).toHaveClass('lg-generate__review-note');
    },
  );

  it('Edge: the review section is described by its note', async () => {
    await reachReviewOfKind('character', 'Review your character');

    const section = screen.getByRole('region', { name: 'Review your character' });
    expect(section).toHaveAttribute('aria-describedby', 'generate-review-note');
    const note = document.getElementById('generate-review-note');
    expect(note).not.toBeNull();
    expect(section).toContainElement(note);
    expect(note).toHaveTextContent(reviewKinds[0].note);
  });

  it('Edge: the note is absent while the generation is still running', async () => {
    renderGenerate({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: ok({
        id: GENERATION_ID,
        status: 'running',
        kind: 'character',
        template_id: TEMPLATE_ID,
        created_at: '2026-11-14T10:00:00Z',
      }),
    });
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
    await userEvent.setup().click(generateButton());

    expect(await screen.findByText(/Generating your character/)).toBeInTheDocument();
    expect(document.querySelector('.lg-generate__review-note')).toBeNull();
    expect(screen.queryByText(reviewKinds[0].note)).toBeNull();
  });

  it('Edge: the note is absent when the generation fails', async () => {
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
    await screen.findByLabelText('Describe your character');
    fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
    await userEvent.setup().click(generateButton());

    expect(await screen.findByText(/We couldn't generate it/)).toBeInTheDocument();
    expect(document.querySelector('.lg-generate__review-note')).toBeNull();
    expect(screen.queryByText(reviewKinds[0].note)).toBeNull();
  });
});
