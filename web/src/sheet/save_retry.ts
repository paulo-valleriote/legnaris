/**
 * Retry policy for a sheet save that lost the server's version race (409).
 *
 * The server never retries a lost compare-and-swap: it answers 409 with a
 * `Retry-After` hint and the client resends, so every retry is a new request the
 * rate limiter accounts for. A 409 is a definite refusal (nothing was written),
 * which is why this is safe where a transport failure on a write is not (API-32).
 */

import { DomainFailure } from '../api/errors';

/** Total sends per save: the first one plus two retries. */
export const SAVE_MAX_ATTEMPTS = 3;

/** Base of the exponential backoff, in milliseconds. */
export const SAVE_BACKOFF_BASE_MS = 250;

/** Upper bound on any single wait, whatever the server hints, in milliseconds. */
export const SAVE_BACKOFF_CAP_MS = 5000;

/** Injectable seams so the schedule is asserted without waiting or randomness. */
export interface SaveRetryDeps {
  /** Resolves after the given delay; defaults to `setTimeout`. */
  readonly sleep?: (ms: number) => Promise<void>;
  /** Uniform value in [0, 1) used as jitter; defaults to `Math.random`. */
  readonly random?: () => number;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** True for the 409 CONFLICT a lost version race produces. */
export function isVersionConflict(error: unknown): boolean {
  return error instanceof DomainFailure && error.kind === 'conflict';
}

/**
 * The wait before retry number [retryIndex] (0-based): the larger of the server
 * hint and the exponential step, plus up to one base step of jitter, capped.
 */
export function conflictDelayMs(
  retryIndex: number,
  hintMs: number | undefined,
  random: () => number,
): number {
  const exponential = SAVE_BACKOFF_BASE_MS * 2 ** retryIndex;
  const jitter = Math.floor(random() * SAVE_BACKOFF_BASE_MS);
  return Math.min(Math.max(hintMs ?? 0, exponential) + jitter, SAVE_BACKOFF_CAP_MS);
}

/**
 * Runs [send], resending after a 409 up to [SAVE_MAX_ATTEMPTS] sends in total.
 * Any other failure, and the last 409, propagates unchanged.
 */
export async function withConflictRetry<T>(
  send: () => Promise<T>,
  deps: SaveRetryDeps = {},
): Promise<T> {
  const sleep = deps.sleep ?? defaultSleep;
  const random = deps.random ?? Math.random;

  for (let attempt = 1; ; attempt += 1) {
    try {
      return await send();
    } catch (error) {
      if (!isVersionConflict(error) || attempt >= SAVE_MAX_ATTEMPTS) {
        throw error;
      }
      const hint = error instanceof DomainFailure ? error.retryAfterMs : undefined;
      await sleep(conflictDelayMs(attempt - 1, hint, random));
    }
  }
}
