/**
 * Narrow decoding helpers shared by the payload models (API-04, API-08, API-09,
 * API-10).
 *
 * Every backend payload is untrusted text until it passes through here: a value
 * of the wrong shape becomes a {@link DefensiveParseFailure} naming the offending
 * key, never a coerced value that would hide contract drift.
 */

import { DefensiveParseFailure } from './errors';

/** True for a non-null, non-array object value. */
export function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** True when [record] has at least one own key. */
export function hasEntries(record: Record<string, unknown>): boolean {
  return Object.keys(record).length > 0;
}

/** Builds the defensive parse failure a caller throws when a payload drifts. */
export function drift(reason: string): DefensiveParseFailure {
  return new DefensiveParseFailure({ reason });
}

/** Reads a required string from [source], naming [key] when it is absent or wrong. */
export function requireString(source: Record<string, unknown>, key: string): string {
  const value = source[key];

  if (typeof value !== 'string') {
    throw drift(`required string "${key}" is missing or not a string`);
  }

  return value;
}

/** Reads a required boolean from [source], naming [key] when it is absent or wrong. */
export function requireBoolean(source: Record<string, unknown>, key: string): boolean {
  const value = source[key];

  if (typeof value !== 'boolean') {
    throw drift(`required boolean "${key}" is missing or not a boolean`);
  }

  return value;
}

/** Reads a required number from [source], naming [key] when it is absent or wrong. */
export function requireNumber(source: Record<string, unknown>, key: string): number {
  const value = source[key];

  if (typeof value !== 'number') {
    throw drift(`required number "${key}" is missing or not a number`);
  }

  return value;
}

/**
 * Reads an optional string from [source].
 *
 * An omitted or null key is `undefined` rather than `''`, so the models keep the
 * backend's absent-versus-empty distinction (API-09).
 */
export function optionalString(source: Record<string, unknown>, key: string): string | undefined {
  const value = source[key];

  if (value === undefined || value === null) {
    return undefined;
  }

  if (typeof value !== 'string') {
    throw drift(`optional string "${key}" is not a string`);
  }

  return value;
}
