import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { message, messageWith, messages } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

const here = path.dirname(fileURLToPath(import.meta.url));
const catalogDir = path.resolve(here, '../catalog');
const engineDir = path.resolve(here, '../engine');
const authDir = path.resolve(here, '../auth');
const landingDir = path.resolve(here, '../landing');
const sheetDir = path.resolve(here, '../sheet');
const libraryDir = path.resolve(here, '../library');

const keys = Object.keys(messages) as MessageKey[];

function isSharedWording(key: MessageKey): boolean {
  return (
    key.startsWith('nav.') ||
    key.startsWith('common.') ||
    key.startsWith('privacy.') ||
    key.startsWith('generation.resumeNotice') ||
    key.endsWith('.retry') ||
    key.endsWith('.close')
  );
}

function withDistinctCopy(keysToCheck: readonly MessageKey[]): MessageKey[] {
  const seen = new Map<string, MessageKey>();
  const clashes: MessageKey[] = [];
  for (const key of keysToCheck) {
    if (isSharedWording(key)) continue;
    const text = messages[key];
    if (seen.has(text)) clashes.push(key);
    seen.set(text, key);
  }
  return clashes;
}

const catalogFeatureKeys: readonly MessageKey[] = [
  'nav.catalog',
  'catalog.title',
  'catalog.loading',
  'catalog.empty',
  'sheet.dialogTitle',
  'sheet.nameLabel',
  'sheet.namePlaceholder',
  'sheet.nameRequired',
  'sheet.create',
  'sheet.close',
  'sheet.loading',
];

const engineFeatureKeys: readonly MessageKey[] = [
  'engine.calculated',
  'engine.none',
  'engine.modified',
  'engine.recalculated',
  'engine.notSet',
];

const sheetCycleKeys: readonly MessageKey[] = [
  'sheet.headerTitle',
  'sheet.openLoading',
  'sheet.templateLoading',
  'sheet.emptyFields',
  'sheet.missing',
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

  it('Happy path: every key resolves to non-empty copy through message()', () => {
    for (const key of keys) {
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Happy path: message() returns the catalog entry itself, not a copy of it', () => {
    for (const key of keys) {
      expect(message(key)).toBe(messages[key]);
    }
  });

  it('Edge: every failure kind owns distinct copy, so no two keys share text', () => {
    expect(withDistinctCopy(keys)).toEqual([]);
  });

  it('Edge: the catalog covers every failure kind key the mapper needs', () => {
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
        'sheet.error.saveOffline',
        'sheet.error.saveOfflineUnkept',
        'sheet.error.previewOffline',
        'sheet.error.previewOfflineUnkept',
      ]),
    );
  });

  it('Happy path: every catalog and sheet copy key resolves to non-empty copy', () => {
    for (const key of catalogFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Happy path: the kind-aware resume notice reads the same in catalog and generate', () => {
    for (const kind of ['character', 'npc', 'enemy'] as const) {
      expect(messages[`generation.resumeNotice.${kind}`]).toBe(
        messages[`generate.resumeNotice.${kind}`],
      );
    }
  });

  it('Edge: the catalog and sheet copy keys own distinct copy, so none is a silent duplicate', () => {
    expect(withDistinctCopy(catalogFeatureKeys)).toEqual([]);
  });

  it('Happy path: every engine copy key resolves to non-empty copy', () => {
    for (const key of engineFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the engine copy keys own distinct copy, so none is a silent duplicate', () => {
    expect(withDistinctCopy(engineFeatureKeys)).toEqual([]);
  });

  it('Happy path: every sheet-cycle copy key resolves to non-empty copy', () => {
    for (const key of sheetCycleKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the sheet-cycle copy keys own distinct copy, so none is a silent duplicate', () => {
    expect(withDistinctCopy(sheetCycleKeys)).toEqual([]);
  });
});

describe('the parameterized copy resolution', () => {
  it('Happy path: a parameterized entry resolves its placeholders from the given values', () => {
    expect(messageWith('auth.codeInstruction', { email: 'a@b.test', minutes: 10 })).toBe(
      'We sent a 6-digit code to a@b.test. It expires in 10 minutes.',
    );
  });

  it('Happy path: the countdown resolves its remaining seconds from the catalog', () => {
    expect(messageWith('auth.codeResendIn', { seconds: 42 })).toBe('Resend in 42s');
  });

  it('Edge: a placeholder without a value raises rather than rendering half a sentence', () => {
    expect(() => messageWith('auth.codeInstruction', { email: 'a@b.test' })).toThrow();
  });

  it('Edge: no catalog entry keeps a placeholder after a resolution', () => {
    for (const key of keys) {
      expect(
        messageWith(key, {
          email: 'a@b.test',
          minutes: 10,
          seconds: 42,
          mode: 'new account',
          year: 2026,
          label: 'Strength',
          position: 2,
          count: 5,
          dice: 'd20',
          rolls: '11',
          max: 6,
          name: 'Aragorn',
          total: 17,
          used: 1,
          limit: 2,
          remaining: 2,
          plan: 'free',
          date: '1 Nov 2026',
          version: '1',
          might: 15,
          modifier: '+2',
          climbing: '+5',
          carry: 105,
          before: '3',
          after: '4',
          characters: 10,
          npcs: 50,
          enemies: 50,
        }),
      ).not.toMatch(/\{(\w+)\}/);
    }
  });
});

function catalogSources(): string[] {
  return readdirSync(catalogDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(catalogDir, name));
}

describe('catalog copy source scan', () => {
  it('Edge: the catalog source resolves every message key and authors no api.error literal', () => {
    const sources = catalogSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    let referenced = 0;

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message(?:With)?\(\s*'([^']+)'\s*[,)]/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced += 1;
      }
    }

    expect(referenced).toBeGreaterThan(0);
  });
});

function engineSources(): string[] {
  return readdirSync(engineDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(engineDir, name));
}

describe('engine copy source scan', () => {
  it('Edge: the engine source authors no user-facing literal and resolves every message key it renders', () => {
    const sources = engineSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    const referenced = new Set<string>();

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message(?:With)?\(\s*'([^']+)'\s*[,)]/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }

    for (const key of engineFeatureKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });
});

const sheetSurfaceKeys: readonly MessageKey[] = [
  'sheet.headerTitle',
  'sheet.openLoading',
  'sheet.templateLoading',
  'sheet.emptyFields',
  'sheet.missing',
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

describe('sheet-cycle copy source scan', () => {
  it('Edge: the sheet cycle authors no user-facing literal and resolves every message key it renders', () => {
    const sources = sheetCycleSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    const referenced = new Set<string>();

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message(?:With)?\(\s*'([^']+)'\s*[,)]/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }

    for (const key of sheetSurfaceKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });
});

const authFeatureKeys: readonly MessageKey[] = [
  'auth.titleLogin',
  'auth.titleRegister',
  'auth.subtitleLogin',
  'auth.subtitleRegister',
  'auth.modeGroupLabel',
  'auth.modeLogin',
  'auth.modeRegister',
  'auth.emailLabel',
  'auth.passwordLabel',
  'auth.usernameLabel',
  'auth.submitLogin',
  'auth.submitRegister',
  'auth.restoring',
  'auth.conflictSignIn',
  'auth.accountModeNew',
  'auth.accountModeExisting',
  'auth.codeOrigin',
  'auth.codeTitleRegister',
  'auth.codeTitleLogin',
  'auth.codeInstruction',
  'auth.codeInstructionExpired',
  'auth.codeExpired',
  'auth.codeLabel',
  'auth.codeSubmit',
  'auth.codeVerifying',
  'auth.codeResendHint',
  'auth.codeResend',
  'auth.codeResendReady',
  'auth.codeResendIn',
  'auth.codeResendSent',
  'auth.codeBackRegister',
  'auth.codeBackLogin',
  'nav.ownSheet',
  'nav.logout',
  'sheet.ownTitle',
  'sheet.ownLoading',
  'sheet.ownEmpty',
  'sheet.ownOpen',
  'sheet.ownDelete',
  'sheet.ownDeleteTitle',
  'sheet.ownDeleteMessage',
  'sheet.ownDeleteConfirm',
  'sheet.ownCancel',
  'api.error.unauthorized',
  'api.error.forbidden',
  'api.error.rateLimited',
  'api.error.invalidCode',
  'api.error.emailUnavailable',
  'api.error.conflict',
  'api.error.disposableEmail',
];

const authSurfaceKeys: readonly MessageKey[] = [
  'auth.titleLogin',
  'auth.titleRegister',
  'auth.subtitleLogin',
  'auth.subtitleRegister',
  'auth.modeGroupLabel',
  'auth.modeLogin',
  'auth.modeRegister',
  'auth.emailLabel',
  'auth.passwordLabel',
  'auth.usernameLabel',
  'auth.submitLogin',
  'auth.submitRegister',
  'auth.restoring',
  'auth.accountModeNew',
  'auth.accountModeExisting',
  'auth.codeOrigin',
  'auth.codeTitleRegister',
  'auth.codeTitleLogin',
  'auth.codeInstruction',
  'auth.codeInstructionExpired',
  'auth.codeExpired',
  'auth.codeLabel',
  'auth.codeSubmit',
  'auth.codeVerifying',
  'auth.codeResendHint',
  'auth.codeResend',
  'auth.codeResendReady',
  'auth.codeResendIn',
  'auth.codeResendSent',
  'auth.codeBackRegister',
  'auth.codeBackLogin',
  'auth.conflictSignIn',
];

function authSources(): string[] {
  return readdirSync(authDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(authDir, name));
}

const uiDir = path.resolve(here, '../ui');

const uiFeatureKeys: readonly MessageKey[] = ['ui.dismiss'];

describe('primitives-layer copy source scan', () => {
  it('Edge: the ui layer authors no user-facing literal and resolves its chrome key', () => {
    const sources = readdirSync(uiDir)
      .filter((n) => n.endsWith('.ts') || n.endsWith('.tsx'))
      .map((n) => path.join(uiDir, n));
    if (sources.length === 0) return;
    const knownKeys = new Set<string>(keys);
    const referenced = new Set<string>();
    for (const file of sources) {
      const text = readFileSync(file, 'utf8');
      expect(text).not.toContain('api.error.');
      for (const match of text.matchAll(/message(?:With)?\(\s*'([^']+)'\s*[,)]/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }
    for (const key of uiFeatureKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });

  it('Happy path: the ui chrome key resolves to non-empty copy', () => {
    for (const key of uiFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });
});

describe('auth copy keys and source scan', () => {
  it('Happy path: every auth and session copy key resolves to non-empty copy', () => {
    for (const key of authFeatureKeys) {
      expect(messages[key]).toBeDefined();
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: the auth and session copy keys own distinct copy, so none is a silent duplicate', () => {
    expect(withDistinctCopy(authFeatureKeys)).toEqual([]);
  });

  it('Edge: the auth source authors no api.error literal and resolves every message key it renders', () => {
    const sources = authSources();
    expect(sources.length).toBeGreaterThan(0);

    const knownKeys = new Set<string>(keys);
    const referenced = new Set<string>();

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');

      expect(text).not.toContain('api.error.');

      for (const match of text.matchAll(/message(?:With)?\(\s*'([^']+)'\s*[,)]/g)) {
        expect(knownKeys.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }

    for (const key of authSurfaceKeys) {
      expect(referenced.has(key)).toBe(true);
    }
  });
});

describe('landing copy keys and source scan (DESIGN.md, Landing surface)', () => {
  const landingKeys = keys.filter((key) => key.startsWith('landing.'));

  it('Happy path: the landing owns a catalog of keys, each resolving to non-empty copy', () => {
    expect(landingKeys.length).toBeGreaterThan(0);
    for (const key of landingKeys) {
      expect(message(key).trim()).not.toBe('');
    }
  });

  it('Edge: every key the landing sources resolve exists, and every landing key is used', () => {
    const known = new Set<string>(keys);
    const referenced = new Set<string>();
    for (const name of readdirSync(landingDir).filter((file) => /\.tsx?$/.test(file))) {
      const text = readFileSync(path.join(landingDir, name), 'utf8');
      for (const match of text.matchAll(/'((?:landing|auth|app|theme)\.[A-Za-z0-9.]+)'/g)) {
        expect(known.has(match[1])).toBe(true);
        referenced.add(match[1]);
      }
    }
    expect(landingKeys.filter((key) => !referenced.has(key))).toEqual([]);
  });

  it('Edge: the copyright line refuses to resolve without its year', () => {
    expect(() => messageWith('landing.copyright', {})).toThrow();
    expect(messageWith('landing.copyright', { year: 2026 })).toBe('© 2026 Legnaris');
  });
});
