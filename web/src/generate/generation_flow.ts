/**
 * The pure rules of the character generation flow (LGN-196, LGN-177 Resolution).
 *
 * Concept bounds, the wait decision and the allowance line are plain functions so
 * every edge (the 9/10/1000/1001 boundaries, the 120 s limit, a missing trial) is
 * asserted without rendering. The screen only maps their answers to copy.
 */

import type { Generation } from '../api/models/generation';

/** The shortest concept the server accepts, in characters (LGN-177). */
export const CONCEPT_MIN_LENGTH = 10;

/** The longest concept the server accepts, in characters (LGN-177). */
export const CONCEPT_MAX_LENGTH = 1000;

/** How often the wait step reads the generation, in milliseconds. */
export const POLL_INTERVAL_MS = 5_000;

/** How long the wait step keeps polling before it gives up, in milliseconds. */
export const WAIT_LIMIT_MS = 120_000;

/** Whether the concept is too short, in range, or too long. */
export type ConceptState = 'tooShort' | 'ok' | 'tooLong';

/**
 * Counts the concept's characters by code point, so an emoji counts once.
 * Leading and trailing whitespace is not part of the concept.
 */
export function conceptLength(text: string): number {
  return Array.from(text.trim()).length;
}

/** Classifies the concept against the 10-1000 bounds. */
export function conceptState(text: string): ConceptState {
  const length = conceptLength(text);
  if (length < CONCEPT_MIN_LENGTH) {
    return 'tooShort';
  }
  if (length > CONCEPT_MAX_LENGTH) {
    return 'tooLong';
  }
  return 'ok';
}

/**
 * What the wait step does with the latest read.
 *
 * `ready` means a generation succeeded and waits to be previewed; `failed` carries
 * the generation's own error code through the caller; `timedOut` is the client's
 * give-up, which is not a failure (the generation keeps running on the server).
 */
export type WaitOutcome = 'keepWaiting' | 'ready' | 'alreadyAccepted' | 'failed' | 'timedOut';

/**
 * Decides the wait step from the latest read and the time spent waiting.
 *
 * A terminal status always wins over the limit, so a generation that succeeds at
 * 119 s or at 130 s is still shown; only a queued or running one gives up at the
 * limit.
 */
export function decideWait(generation: Generation | undefined, elapsedMs: number): WaitOutcome {
  if (generation === undefined) {
    return elapsedMs >= WAIT_LIMIT_MS ? 'timedOut' : 'keepWaiting';
  }
  switch (generation.status) {
    case 'succeeded':
      return 'ready';
    case 'accepted':
      return 'alreadyAccepted';
    case 'failed':
      return 'failed';
    case 'queued':
    case 'running':
      return elapsedMs >= WAIT_LIMIT_MS ? 'timedOut' : 'keepWaiting';
  }
}

/** The allowance the quota line shows: a Free trial count, or unknown when the payload has none. */
export type Allowance =
  { readonly kind: 'trial'; readonly remaining: number } | { readonly kind: 'unknown' };

/** Reads the allowance from the plan's trial figure; a missing figure is unknown, never zero. */
export function trialAllowance(plan: { readonly trialRemaining: number | undefined }): Allowance {
  if (plan.trialRemaining === undefined) {
    return { kind: 'unknown' };
  }
  return { kind: 'trial', remaining: plan.trialRemaining };
}
