import { DomainFailure } from '../api/errors';

export const SAVE_MAX_ATTEMPTS = 3;

export const SAVE_BACKOFF_BASE_MS = 250;

export const SAVE_BACKOFF_CAP_MS = 5000;

export interface SaveRetryDeps {
  readonly sleep?: (ms: number) => Promise<void>;
  readonly random?: () => number;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function isVersionConflict(error: unknown): boolean {
  return error instanceof DomainFailure && error.kind === 'conflict';
}

export function conflictDelayMs(
  retryIndex: number,
  hintMs: number | undefined,
  random: () => number,
): number {
  const exponential = SAVE_BACKOFF_BASE_MS * 2 ** retryIndex;
  const jitter = Math.floor(random() * SAVE_BACKOFF_BASE_MS);
  return Math.min(Math.max(hintMs ?? 0, exponential) + jitter, SAVE_BACKOFF_CAP_MS);
}

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
