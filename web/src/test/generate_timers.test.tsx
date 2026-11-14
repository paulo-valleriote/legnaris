import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen } from '@testing-library/react';
import { POLL_INTERVAL_MS, WAIT_LIMIT_MS } from '../generate/generation_flow';
import {
  GENERATION_ID,
  VALID_CONCEPT,
  conceptField,
  countOf,
  generateButton,
  ok,
  renderGenerate,
} from './generate_fixtures';
import { TEMPLATE_ID } from './sheet_fixtures';

const TIMEOUT_COPY = 'This is taking longer than usual.';

/** Lets pending promises and the fake clock settle for `ms` milliseconds. */
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

function generation(status: string, values?: Record<string, unknown>) {
  return ok({
    id: GENERATION_ID,
    status,
    kind: 'character',
    template_id: TEMPLATE_ID,
    ...(values === undefined ? {} : { values }),
    created_at: '2026-11-14T10:00:00Z',
  });
}

/** Renders the concept, submits it, and returns the clock reading at submission. */
async function submit(routes: Parameters<typeof renderGenerate>[0]) {
  const fake = renderGenerate(routes);
  await advance(0);
  fireEvent.change(conceptField(), { target: { value: VALID_CONCEPT } });
  fireEvent.click(generateButton());
  await advance(0);
  return fake;
}

describe('generate wait clock (LGN-196, fake timers)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('Edge: a generation still running at 119.9 s keeps waiting; at 120 s it gives up', async () => {
    const fake = await submit({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: generation('running'),
    });

    await advance(WAIT_LIMIT_MS - 100);
    expect(screen.queryByText(TIMEOUT_COPY, { exact: false })).toBeNull();

    await advance(100);
    expect(screen.getByText(TIMEOUT_COPY, { exact: false })).toBeInTheDocument();
    expect(countOf(fake.requests, 'GET', `/generations/${GENERATION_ID}`)).toBe(
      WAIT_LIMIT_MS / POLL_INTERVAL_MS + 1,
    );
  });

  it('Edge: the give-up is not a failure, so no failure copy is shown and no refund is claimed', async () => {
    await submit({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: generation('running'),
    });

    await advance(WAIT_LIMIT_MS);

    expect(screen.queryByText(/We couldn't generate it/)).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('Edge: a generation that succeeds just before the limit is reviewed, not given up', async () => {
    let succeedAt = Number.POSITIVE_INFINITY;
    await submit({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: () =>
        Date.now() >= succeedAt ? generation('succeeded', {}) : generation('running'),
    });
    succeedAt = Date.now() + (WAIT_LIMIT_MS - 5_000);

    await advance(WAIT_LIMIT_MS);

    expect(screen.getByRole('heading', { name: 'Review your character' })).toBeInTheDocument();
    expect(screen.queryByText(TIMEOUT_COPY, { exact: false })).toBeNull();
  });

  it('Happy path: timeout then late success resumes the same generation and reviews it', async () => {
    let status = 'running';
    const fake = await submit({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: () => generation(status, {}),
    });
    await advance(WAIT_LIMIT_MS);
    expect(screen.getByText(TIMEOUT_COPY, { exact: false })).toBeInTheDocument();

    status = 'succeeded';
    fireEvent.click(screen.getByRole('button', { name: 'Keep waiting' }));
    await advance(0);

    expect(screen.getByRole('heading', { name: 'Review your character' })).toBeInTheDocument();
    expect(countOf(fake.requests, 'POST', '/generations')).toBe(1);
  });

  it('Edge: keep waiting restarts the clock, so the next give-up is a full window away', async () => {
    await submit({
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: generation('running'),
    });
    await advance(WAIT_LIMIT_MS);
    fireEvent.click(screen.getByRole('button', { name: 'Keep waiting' }));
    await advance(0);

    await advance(WAIT_LIMIT_MS - POLL_INTERVAL_MS);
    expect(screen.queryByText(TIMEOUT_COPY, { exact: false })).toBeNull();

    await advance(POLL_INTERVAL_MS);
    expect(screen.getByText(TIMEOUT_COPY, { exact: false })).toBeInTheDocument();
  });
});
