import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { message, messages } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

const here = path.dirname(fileURLToPath(import.meta.url));
const catalogDir = path.resolve(here, '../catalog');
const engineDir = path.resolve(here, '../engine');
const authDir = path.resolve(here, '../auth');
const sheetDir = path.resolve(here, '../sheet');
const libraryDir = path.resolve(here, '../library');

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

/** The sheet-cycle copy the open, edit, draft and guard surfaces render (WSC-32, SC-006). */
const sheetCycleKeys: readonly MessageKey[] = [
  'sheet.headerTitle',
  'sheet.openLoading',
  'sheet.templateLoading',
  'sheet.emptyFields',
  'sheet.missing',
  'sheet.retryOpen',
  'sheet.pendingEdits',
  'sheet.save',
  'sheet.saving',
  'sheet.saved',
  'sheet.creating',
  'sheet.preview',
  'sheet.previewing',
  'sheet.refreshTitle',
  'sheet.refreshMessage',
  'sheet.refreshConfirm',
  'sheet.leaveTitle',
  'sheet.leaveMessage',
  'sheet.leaveConfirm',
  'sheet.cancel',
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

  it('Happy path: every sheet-cycle copy key resolves to non-empty copy (WSC-32, SC-006)', () => {
    for (const key of sheetCycleKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the sheet-cycle copy keys own distinct copy, so none is a silent duplicate (WSC-32)', () => {
    const seen = new Map<string, MessageKey>();
    for (const key of sheetCycleKeys) {
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

/** The keys the sheet cycle renders in `sheet/` (the F3 dialog owns `sheet.creating`). */
const sheetSurfaceKeys: readonly MessageKey[] = [
  'sheet.headerTitle',
  'sheet.openLoading',
  'sheet.templateLoading',
  'sheet.emptyFields',
  'sheet.missing',
  'sheet.retryOpen',
  'sheet.pendingEdits',
  'sheet.save',
  'sheet.saving',
  'sheet.saved',
  'sheet.preview',
  'sheet.previewing',
  'sheet.refreshTitle',
  'sheet.refreshMessage',
  'sheet.refreshConfirm',
  'sheet.leaveTitle',
  'sheet.leaveMessage',
  'sheet.leaveConfirm',
  'sheet.cancel',
];

function sheetCycleSources(): string[] {
  const scan = (dir: string) =>
    readdirSync(dir)
      .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
      .map((name) => path.join(dir, name));
  return [
    ...scan(sheetDir),
    ...scan(libraryDir),
    path.resolve(here, '../shell/shortcut_registry.ts'),
  ];
}

describe('sheet-cycle copy source scan (WSC-32, SC-006)', () => {
  it('Edge: the sheet cycle authors no user-facing literal and resolves every message key it renders', () => {
    const sources = sheetCycleSources();
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

    // Every sheet surface key the catalog declares must be rendered somewhere in
    // the cycle, so dropping a reference turns this suite red rather than pass
    // vacuously (SC-006).
    for (const key of sheetSurfaceKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });
});

/** The auth surface and session copy the feature introduces (WAS-19, WAS-25, WAS-30, WAS-31). */
const authFeatureKeys: readonly MessageKey[] = [
  'auth.title',
  'auth.modeLogin',
  'auth.modeRegister',
  'auth.emailLabel',
  'auth.passwordLabel',
  'auth.usernameLabel',
  'auth.submitLogin',
  'auth.submitRegister',
  'auth.restoring',
  'nav.ownSheet',
  'nav.logout',
  'sheet.ownTitle',
  'sheet.ownLoading',
  'sheet.ownEmpty',
  'sheet.ownOpen',
  'sheet.ownRetry',
  'sheet.ownDelete',
  'sheet.ownDeleteTitle',
  'sheet.ownDeleteMessage',
  'sheet.ownDeleteConfirm',
  'sheet.ownCancel',
  'api.error.unauthorized',
  'api.error.forbidden',
  'api.error.rateLimited',
];

/** The keys the auth directory renders; the rest are owned by the shell and the sheet surface. */
const authSurfaceKeys: readonly MessageKey[] = [
  'auth.title',
  'auth.modeLogin',
  'auth.modeRegister',
  'auth.emailLabel',
  'auth.passwordLabel',
  'auth.usernameLabel',
  'auth.submitLogin',
  'auth.submitRegister',
  'auth.restoring',
];

function authSources(): string[] {
  return readdirSync(authDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(authDir, name));
}

describe('auth copy keys and source scan (WAS-19, WAS-25, WAS-30, WAS-31)', () => {
  it('Happy path: every auth and session copy key resolves to non-empty copy', () => {
    for (const key of authFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the auth and session copy keys own distinct copy, so none is a silent duplicate', () => {
    const seen = new Map<string, MessageKey>();
    for (const key of authFeatureKeys) {
      const text = messages[key];
      expect(seen.has(text)).toBe(false);
      seen.set(text, key);
    }
  });

  it('Edge: the auth source authors no api.error literal and resolves every message key it renders', () => {
    const sources = authSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    const referenced = new Set<string>();

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      // Failure copy is resolved through the mapper, never by naming a key here.
      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message\(\s*'([^']+)'\s*\)/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }

    // Every auth-surface key the catalog declares must be rendered somewhere in
    // the directory, so dropping a reference turns this suite red (WAS-31).
    for (const key of authSurfaceKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });
});
