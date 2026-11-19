import { beforeAll, describe, expect, it } from 'vitest';
import {
  STORAGE_MESSAGE,
  hasMessage,
  messagesAt,
  restricted,
  ruleIds,
  warmUpEslint,
} from './eslint_harness';

beforeAll(warmUpEslint, 60000);

describe('ESLint storage seam (LGN-290)', () => {
  it('Error: bare localStorage and sessionStorage are refused outside the seams', async () => {
    for (const name of ['localStorage', 'sessionStorage']) {
      const messages = await messagesAt(
        'src/sheet/probe.ts',
        `export const value = ${name}.getItem('k');`,
      );

      expect(ruleIds(messages)).toContain('no-restricted-globals');
      expect(hasMessage(messages, STORAGE_MESSAGE)).toBe(true);
    }
  });

  it('Error: window, self and globalThis storage properties are refused outside the seams', async () => {
    for (const target of ['window', 'self', 'globalThis']) {
      const messages = await messagesAt(
        'src/sheet/probe.ts',
        `export const value = ${target}.localStorage.getItem('k');`,
      );

      expect(ruleIds(messages)).toContain('no-restricted-properties');
      expect(hasMessage(messages, STORAGE_MESSAGE)).toBe(true);
    }
  });

  it('Happy path: the theme and view-state seams may read and write storage', async () => {
    const code = [
      "export const read = window.localStorage.getItem('k');",
      "export const write = sessionStorage.setItem('k', 'v');",
    ].join('\n');

    for (const file of ['src/theme/use_theme.ts', 'src/sheet/view_store.ts']) {
      expect(restricted(await messagesAt(file, code))).toEqual([]);
    }
  });
});

describe('ESLint IndexedDB import seam (LGN-290)', () => {
  it('Error: idb may not be imported outside the IndexedDB sheet store', async () => {
    const messages = await messagesAt(
      'src/sheet/probe.ts',
      "import { openDB } from 'idb';\nexport const open = openDB;",
    );

    expect(ruleIds(messages)).toContain('no-restricted-imports');
  });

  it('Happy path: the IndexedDB sheet store may import idb', async () => {
    const code = "import { openDB } from 'idb';\nexport const open = openDB;";

    expect(restricted(await messagesAt('src/library/idb_sheet_store.ts', code))).toEqual([]);
  });
});
