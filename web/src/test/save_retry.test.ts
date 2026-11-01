import { describe, expect, it, vi } from 'vitest';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import {
  SAVE_BACKOFF_BASE_MS,
  SAVE_BACKOFF_CAP_MS,
  SAVE_MAX_ATTEMPTS,
  conflictDelayMs,
  isVersionConflict,
  withConflictRetry,
} from '../sheet/save_retry';

function conflict(retryAfterMs?: number): DomainFailure {
  return new DomainFailure({ kind: 'conflict', status: 409, code: 'CONFLICT', retryAfterMs });
}

describe('conflictDelayMs', () => {
  it('Happy path: grows exponentially from the base with no hint and no jitter', () => {
    const zero = () => 0;

    expect(conflictDelayMs(0, undefined, zero)).toBe(SAVE_BACKOFF_BASE_MS);
    expect(conflictDelayMs(1, undefined, zero)).toBe(SAVE_BACKOFF_BASE_MS * 2);
  });

  it('Edge: a larger server hint wins over the exponential step', () => {
    expect(conflictDelayMs(0, 2000, () => 0)).toBe(2000);
  });

  it('Edge: a smaller server hint never shortens the exponential step', () => {
    expect(conflictDelayMs(1, 10, () => 0)).toBe(SAVE_BACKOFF_BASE_MS * 2);
  });

  it('Edge: jitter adds strictly less than one base step', () => {
    expect(conflictDelayMs(0, undefined, () => 0.999)).toBeLessThan(SAVE_BACKOFF_BASE_MS * 2);
  });

  it('Edge: a huge hint is capped', () => {
    expect(conflictDelayMs(0, 3_600_000, () => 0.5)).toBe(SAVE_BACKOFF_CAP_MS);
  });
});

describe('isVersionConflict', () => {
  it('Happy path: true only for a conflict domain failure', () => {
    expect(isVersionConflict(conflict())).toBe(true);
  });

  it.each([
    ['another domain kind', new DomainFailure({ kind: 'rateLimited', status: 429 })],
    ['a parse failure', new DefensiveParseFailure({ reason: 'x' })],
    ['a plain error', new Error('x')],
    ['undefined', undefined],
  ])('Edge: false for %s', (_name, error) => {
    expect(isVersionConflict(error)).toBe(false);
  });
});

describe('withConflictRetry', () => {
  const deps = () => ({ sleep: vi.fn().mockResolvedValue(undefined), random: () => 0 });

  it('Happy path: a first-try success sends once and never sleeps', async () => {
    const d = deps();
    const send = vi.fn().mockResolvedValue('ok');

    await expect(withConflictRetry(send, d)).resolves.toBe('ok');

    expect(send).toHaveBeenCalledTimes(1);
    expect(d.sleep).not.toHaveBeenCalled();
  });

  it('Happy path: a 409 is resent after the hinted wait and then succeeds', async () => {
    const d = deps();
    const send = vi.fn().mockRejectedValueOnce(conflict(1000)).mockResolvedValue('ok');

    await expect(withConflictRetry(send, d)).resolves.toBe('ok');

    expect(send).toHaveBeenCalledTimes(2);
    expect(d.sleep).toHaveBeenCalledExactlyOnceWith(1000);
  });

  it('Error: persistent 409 stops after the bounded attempts and rethrows the conflict', async () => {
    const d = deps();
    const failure = conflict(1000);
    const send = vi.fn().mockRejectedValue(failure);

    await expect(withConflictRetry(send, d)).rejects.toBe(failure);

    expect(send).toHaveBeenCalledTimes(SAVE_MAX_ATTEMPTS);
    expect(d.sleep).toHaveBeenCalledTimes(SAVE_MAX_ATTEMPTS - 1);
  });

  it('Edge: waits grow between retries when the server sends no hint', async () => {
    const d = deps();
    const send = vi.fn().mockRejectedValue(conflict());

    await expect(withConflictRetry(send, d)).rejects.toBeInstanceOf(DomainFailure);

    expect(d.sleep.mock.calls.map(([ms]) => ms)).toEqual([
      SAVE_BACKOFF_BASE_MS,
      SAVE_BACKOFF_BASE_MS * 2,
    ]);
  });

  it.each([
    ['a server error', new DomainFailure({ kind: 'serverError', status: 500 })],
    ['a rate limit', new DomainFailure({ kind: 'rateLimited', status: 429 })],
    ['an offline failure', new DomainFailure({ kind: 'offline' })],
    ['a parse failure', new DefensiveParseFailure({ reason: 'x' })],
  ])('Error: %s is never resent', async (_name, failure) => {
    const d = deps();
    const send = vi.fn().mockRejectedValue(failure);

    await expect(withConflictRetry(send, d)).rejects.toBe(failure);

    expect(send).toHaveBeenCalledTimes(1);
    expect(d.sleep).not.toHaveBeenCalled();
  });
});
