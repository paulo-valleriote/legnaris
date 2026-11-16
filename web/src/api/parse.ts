import { DefensiveParseFailure } from './errors';

export function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function hasEntries(record: Record<string, unknown>): boolean {
  return Object.keys(record).length > 0;
}

export function drift(reason: string): DefensiveParseFailure {
  return new DefensiveParseFailure({ reason });
}

export function requireString(source: Record<string, unknown>, key: string): string {
  const value = source[key];

  if (typeof value !== 'string') {
    throw drift(`required string "${key}" is missing or not a string`);
  }

  return value;
}

export function requireBoolean(source: Record<string, unknown>, key: string): boolean {
  const value = source[key];

  if (typeof value !== 'boolean') {
    throw drift(`required boolean "${key}" is missing or not a boolean`);
  }

  return value;
}

export function requireNumber(source: Record<string, unknown>, key: string): number {
  const value = source[key];

  if (typeof value !== 'number') {
    throw drift(`required number "${key}" is missing or not a number`);
  }

  return value;
}

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
