import { describe, expect, it } from 'vitest';
import { message, messages } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

const keys = Object.keys(messages) as MessageKey[];

describe('translation catalog', () => {
  it('Edge: the catalog is not empty', () => {
    expect(keys.length).toBeGreaterThan(0);
  });

  it('Happy path: every key resolves to non-empty copy through message() (API-23)', () => {
    for (const key of keys) {
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Happy path: message() returns the catalog entry itself, not a copy of it', () => {
    for (const key of keys) {
      expect(message(key)).toBe(messages[key]);
    }
  });

  it('Edge: every failure kind owns distinct copy, so no two keys share text (API-23)', () => {
    const copy = keys.map((key) => messages[key]);
    expect(new Set(copy).size).toBe(copy.length);
  });

  it('Edge: the catalog covers every failure kind key the mapper needs (API-23)', () => {
    expect(keys).toEqual(
      expect.arrayContaining([
        'api.error.clientFault',
        'api.error.invalidEdit',
        'api.error.renderMismatch',
        'api.error.notFound',
        'api.error.serverError',
        'api.error.offline',
        'api.error.unknown',
        'api.error.parse',
      ]),
    );
  });
});
