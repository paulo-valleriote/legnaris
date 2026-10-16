import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { message, messages } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

const here = path.dirname(fileURLToPath(import.meta.url));
const catalogDir = path.resolve(here, '../catalog');
const engineDir = path.resolve(here, '../engine');

const keys = Object.keys(messages) as MessageKey[];

/** The catalog and creation-surface copy F3 introduces (WCT-26, WCT-29). */
const catalogFeatureKeys: readonly MessageKey[] = [
  'nav.catalog',
  'catalog.title',
  'catalog.loading',
  'catalog.empty',
  'catalog.retry',
  'catalog.openHint',
  'sheet.dialogTitle',
  'sheet.nameLabel',
  'sheet.namePlaceholder',
  'sheet.nameRequired',
  'sheet.create',
  'sheet.close',
  'sheet.loading',
  'sheet.retry',
];

/** The widget-level copy the UI engine renders (ENG-27, SC-003). */
const engineFeatureKeys: readonly MessageKey[] = [
  'engine.calculated',
  'engine.none',
  'engine.modified',
  'engine.recalculated',
];

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

  it('Happy path: every catalog and sheet copy key resolves to non-empty copy (WCT-26, WCT-29)', () => {
    for (const key of catalogFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the catalog and sheet copy keys own distinct copy, so none is a silent duplicate (WCT-26)', () => {
    const seen = new Map<string, MessageKey>();
    for (const key of catalogFeatureKeys) {
      const text = messages[key];
      expect(seen.has(text)).toBe(false);
      seen.set(text, key);
    }
  });

  it('Happy path: every engine copy key resolves to non-empty copy (ENG-27, SC-003)', () => {
    for (const key of engineFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the engine copy keys own distinct copy, so none is a silent duplicate (ENG-27)', () => {
    const seen = new Map<string, MessageKey>();
    for (const key of engineFeatureKeys) {
      const text = messages[key];
      expect(seen.has(text)).toBe(false);
      seen.set(text, key);
    }
  });
});

function catalogSources(): string[] {
  return readdirSync(catalogDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(catalogDir, name));
}

describe('catalog copy source scan (WCT-26, SC-003, SC-004)', () => {
  it('Edge: the catalog source resolves every message key and authors no api.error literal', () => {
    const sources = catalogSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    let referenced = 0;

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      // Failure copy is resolved through the mapper, never by naming a key here.
      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message\(\s*'([^']+)'\s*\)/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced += 1;
      }
    }

    // Removing a referenced key must turn this suite red rather than pass vacuously.
    expect(referenced).toBeGreaterThan(0);
  });
});

function engineSources(): string[] {
  return readdirSync(engineDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(engineDir, name));
}

describe('engine copy source scan (ENG-27, SC-003, SC-004)', () => {
  it('Edge: the engine source authors no user-facing literal and resolves every message key it renders', () => {
    const sources = engineSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    const referenced = new Set<string>();

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      // Copy is resolved through message(), never by naming a failure key here.
      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message\(\s*'([^']+)'\s*\)/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }

    // Every engine key the catalog declares must be rendered somewhere, so
    // dropping a reference turns this suite red rather than pass vacuously.
    for (const key of engineFeatureKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });
});
