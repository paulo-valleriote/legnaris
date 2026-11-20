import type { Generation } from '../api/models/generation';
import { SHEET_KINDS } from '../api/models/plan';
import type { SheetKind } from '../api/models/plan';

export const PREVIEW_THROTTLE_MS = 1_000;

const UNSAFE_KEYS: ReadonlySet<string> = new Set(['__proto__', 'constructor', 'prototype']);

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

export function isFreePlan(plan: { readonly plan: string } | undefined): boolean {
  return plan !== undefined && plan.plan === 'free';
}

export const CONCEPT_MIN_LENGTH = 10;

export const CONCEPT_MAX_LENGTH = 1000;

export const POLL_INTERVAL_MS = 5_000;

export const WAIT_LIMIT_MS = 120_000;

export type ConceptState = 'tooShort' | 'ok' | 'tooLong';

const GO_SPACE_CODE_POINTS = new Set([
  0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000,
]);

function isGoSpace(char: string): boolean {
  const codePoint = char.codePointAt(0) ?? 0;
  return GO_SPACE_CODE_POINTS.has(codePoint) || (codePoint >= 0x2000 && codePoint <= 0x200a);
}

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

export function conceptLength(text: string): number {
  return Array.from(trimConcept(text)).length;
}

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

export type WaitOutcome = 'keepWaiting' | 'ready' | 'alreadyAccepted' | 'failed' | 'timedOut';

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

export function generationKind(param: string | null | undefined): SheetKind {
  return SHEET_KINDS.find((kind) => kind === param) ?? 'character';
}
