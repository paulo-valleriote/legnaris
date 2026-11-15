/**
 * The pure rules of the character generation flow (LGN-196, LGN-177 Resolution).
 *
 * Concept bounds, the wait decision and the allowance line are plain functions so
 * every edge (the 9/10/1000/1001 boundaries, the 120 s limit, a missing trial) is
 * asserted without rendering. The screen only maps their answers to copy.
 */

import type { Generation } from '../api/models/generation';
import type { FieldNode } from '../api/models/template';

/** The shortest gap between two derived-value previews, in milliseconds (LGN-196). */
export const PREVIEW_THROTTLE_MS = 1_000;

/** The field path a generated character name is read from; the fantasy template names it so. */
export const NAME_LEAF_PATH = 'identity.characterName';

/** Keys a merge never writes, so a crafted cascade cannot reach the prototype chain. */
const UNSAFE_KEYS: ReadonlySet<string> = new Set(['__proto__', 'constructor', 'prototype']);

/**
 * How long to wait before the next derived-value preview may be sent.
 *
 * Zero when none was sent yet or a full window has passed; otherwise the rest of
 * the window. A clock that went backwards waits the full window, never longer.
 */
export function previewDelay(lastSentAt: number | undefined, now: number): number {
  if (lastSentAt === undefined) {
    return 0;
  }
  const elapsed = now - lastSentAt;
  if (elapsed < 0) {
    return PREVIEW_THROTTLE_MS;
  }
  return Math.max(0, PREVIEW_THROTTLE_MS - elapsed);
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Merges the recalculated cascade over the values, returning a new object.
 *
 * Only the paths the cascade names change; nested objects merge key by key, and
 * arrays or scalars replace. Unsafe keys are dropped, and a non-object cascade
 * changes nothing.
 */
export function mergeCascade(
  values: Record<string, unknown>,
  cascade: Record<string, unknown>,
): Record<string, unknown> {
  if (!isPlainRecord(cascade)) {
    return values;
  }
  const merged: Record<string, unknown> = { ...values };
  for (const key of Object.keys(cascade)) {
    if (UNSAFE_KEYS.has(key)) {
      continue;
    }
    const incoming = cascade[key];
    const current = merged[key];
    merged[key] =
      isPlainRecord(incoming) && isPlainRecord(current)
        ? mergeCascade(current, incoming)
        : incoming;
  }
  return merged;
}

/**
 * The Sheet name to prefill from the generated character name, or an empty string.
 *
 * Read only when the template declares the name leaf and the generated value is a
 * non-blank string; anything else leaves the name for the player to type.
 */
export function suggestedSheetName(
  fields: Readonly<Record<string, FieldNode>>,
  values: Record<string, unknown>,
): string {
  const segments = NAME_LEAF_PATH.split('.');
  if (!hasLeafAt(fields, segments)) {
    return '';
  }
  const value = readAt(values, segments);
  return typeof value === 'string' ? value.trim() : '';
}

function hasLeafAt(
  fields: Readonly<Record<string, FieldNode>>,
  segments: readonly string[],
): boolean {
  let level: Readonly<Record<string, FieldNode>> | undefined = fields;
  for (let index = 0; index < segments.length; index += 1) {
    const node: FieldNode | undefined = level?.[segments[index] ?? ''];
    if (node === undefined) {
      return false;
    }
    if (index === segments.length - 1) {
      return node.kind === 'leaf';
    }
    level = node.kind === 'group' ? node.children : undefined;
  }
  return false;
}

function readAt(values: Record<string, unknown>, segments: readonly string[]): unknown {
  let current: unknown = values;
  for (const segment of segments) {
    if (!isPlainRecord(current)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

/**
 * Whether the next regenerate spends the last free generation (LGN-196). The
 * server decides; a plan not yet read never asks.
 */
export function isLastGeneration(
  plan: { readonly lastFreeGeneration: boolean } | undefined,
): boolean {
  return plan?.lastFreeGeneration === true;
}

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
 * The code points Go's unicode.IsSpace accepts. The server trims the concept with
 * strings.TrimSpace, which uses this set, so the client must not use String#trim:
 * that trims U+FEFF and keeps U+0085, the opposite of the server.
 */
const GO_SPACE_CODE_POINTS = new Set([
  0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000,
]);

function isGoSpace(char: string): boolean {
  const codePoint = char.codePointAt(0) ?? 0;
  return GO_SPACE_CODE_POINTS.has(codePoint) || (codePoint >= 0x2000 && codePoint <= 0x200a);
}

/**
 * Removes the leading and trailing characters the server trims, with the same
 * set as Go's strings.TrimSpace, so the concept sent and the concept counted match
 * what the server stores.
 */
export function trimConcept(text: string): string {
  const chars = Array.from(text);
  let start = 0;
  let end = chars.length;
  while (start < end && isGoSpace(chars[start])) {
    start += 1;
  }
  while (end > start && isGoSpace(chars[end - 1])) {
    end -= 1;
  }
  return chars.slice(start, end).join('');
}

/**
 * Counts the concept's characters by code point, so an emoji counts once.
 * Leading and trailing whitespace is not part of the concept.
 */
export function conceptLength(text: string): number {
  return Array.from(trimConcept(text)).length;
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
