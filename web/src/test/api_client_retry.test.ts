import { afterEach, describe, expect, it, vi } from 'vitest';
import { DEFAULT_REQUEST_TIMEOUT_MS, DEFAULT_RETRY_DELAY_MS, createApiClient } from '../api/client';
import type { ApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import type { Transport } from '../api/transport';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';

function recordingSleep() {
  const delays: number[] = [];

  return {
    delays,
    sleep: (ms: number) => {
      delays.push(ms);
      return Promise.resolve();
    },
  };
}

function clientWith(transport: Transport, sleep?: (ms: number) => Promise<void>): ApiClient {
  return createApiClient({ baseUrl, transport, ...(sleep === undefined ? {} : { sleep }) });
}

describe('bounded attempts', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('Happy path: a read failing at the transport is retried exactly once after the fixed delay (API-31)', async () => {
    const fake = createFakeTransport(
      { failure: new Error('network') },
      successOutcome({ status: 'ok' }),
    );
    const recorder = recordingSleep();

    const payload = await clientWith(fake.transport, recorder.sleep).health();

    expect(payload).toEqual({ status: 'ok' });
    expect(fake.attempts).toBe(2);
    expect(recorder.delays).toEqual([DEFAULT_RETRY_DELAY_MS]);
  });

  it('Edge: a read failing twice ends as offline and is never attempted a third time (API-20, API-31)', async () => {
    const fake = createFakeTransport(
      { failure: new Error('network') },
      { failure: new Error('network') },
    );
    const recorder = recordingSleep();

    const failure = await clientWith(fake.transport, recorder.sleep)
      .health()
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({ kind: 'offline' });
    expect(fake.attempts).toBe(2);
    expect(recorder.delays).toEqual([DEFAULT_RETRY_DELAY_MS]);
  });

  it('Edge: a read answered with a failure status is attempted once and surfaced immediately (API-32)', async () => {
    const fake = createFakeTransport(errorOutcome('INTERNAL_ERROR', 'boom', 500));
    const recorder = recordingSleep();

    const failure = await clientWith(fake.transport, recorder.sleep)
      .listTemplates()
      .catch((error: unknown) => error);

    expect(failure).toMatchObject({ kind: 'serverError', status: 500 });
    expect(fake.attempts).toBe(1);
    expect(recorder.delays).toEqual([]);
  });

  it('Edge: a write failing at the transport is never retried (API-33)', async () => {
    const fake = createFakeTransport({ failure: new Error('network') });
    const recorder = recordingSleep();

    const failure = await clientWith(fake.transport, recorder.sleep)
      .createSheet({ templateId: 'fantasy_v1', sheetName: 'Aragorn' })
      .catch((error: unknown) => error);

    expect(failure).toMatchObject({ kind: 'offline' });
    expect(fake.attempts).toBe(1);
    expect(recorder.delays).toEqual([]);
  });

  it('Edge: a write answered with a failure status is never retried (API-33)', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'refused', 400));
    const recorder = recordingSleep();

    const failure = await clientWith(fake.transport, recorder.sleep)
      .patchSheet('sheet-1', { stats: { dexterity: 12 } })
      .catch((error: unknown) => error);

    expect(failure).toMatchObject({ kind: 'invalidEdit', status: 400 });
    expect(fake.attempts).toBe(1);
    expect(recorder.delays).toEqual([]);
  });

  it('Edge: the retry delay is injectable and its value is asserted, not waited for (API-31)', async () => {
    const fake = createFakeTransport(
      { failure: new Error('network') },
      successOutcome({ status: 'ok' }),
    );
    const recorder = recordingSleep();

    await createApiClient({
      baseUrl,
      transport: fake.transport,
      retryDelayMs: 42,
      sleep: recorder.sleep,
    }).health();

    expect(recorder.delays).toEqual([42]);
  });

  it('Error: an unreadable response ends the attempt without retrying (API-32)', async () => {
    const fake = createFakeTransport({ status: 200, body: 'not json' });
    const recorder = recordingSleep();

    const failure = await clientWith(fake.transport, recorder.sleep)
      .health()
      .catch((error: unknown) => error);

    expect(failure).not.toBeInstanceOf(DomainFailure);
    expect(fake.attempts).toBe(1);
    expect(recorder.delays).toEqual([]);
  });

  it('Edge: an attempt is aborted at the five-second default timeout (API-30)', async () => {
    vi.useFakeTimers();
    const signals: AbortSignal[] = [];
    const transport: Transport = (request) => {
      signals.push(request.signal);
      return new Promise((_resolve, reject) => {
        request.signal.addEventListener('abort', () => reject(new Error('aborted')));
      });
    };

    const pending = clientWith(transport)
      .health()
      .catch((error: unknown) => error);

    await vi.advanceTimersByTimeAsync(DEFAULT_REQUEST_TIMEOUT_MS);
    expect(signals[0]?.aborted).toBe(true);

    await vi.advanceTimersByTimeAsync(DEFAULT_RETRY_DELAY_MS);
    await vi.advanceTimersByTimeAsync(DEFAULT_REQUEST_TIMEOUT_MS);

    const failure = await pending;
    expect(failure).toMatchObject({ kind: 'offline' });
    expect(signals).toHaveLength(2);
  });

  it('Edge: a custom timeout bounds each attempt instead of the default (API-30)', async () => {
    vi.useFakeTimers();
    const signals: AbortSignal[] = [];
    const transport: Transport = (request) => {
      signals.push(request.signal);
      return new Promise((_resolve, reject) => {
        request.signal.addEventListener('abort', () => reject(new Error('aborted')));
      });
    };

    void createApiClient({ baseUrl, transport, timeoutMs: 1000 })
      .health()
      .catch(() => undefined);

    await vi.advanceTimersByTimeAsync(999);
    expect(signals[0]?.aborted).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(signals[0]?.aborted).toBe(true);
  });

  it('Edge: the default timeout and retry delay are the contracted values (API-30, API-31)', () => {
    expect(DEFAULT_REQUEST_TIMEOUT_MS).toBe(5000);
    expect(DEFAULT_RETRY_DELAY_MS).toBe(300);
  });
});
