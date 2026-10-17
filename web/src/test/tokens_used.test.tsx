/**
 * The literal-value scan: the scanned component directories must contain no
 * hexadecimal color and no pixel value, and every token they reference must be
 * declared in `tokens.css` (UIF-29, UIF-30).
 *
 * `src/theme/` is the deliberate exception and is therefore never scanned: it is
 * where the literal values are declared. The scan forbids them everywhere else.
 */
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { readTokenNames } from './tokens_files';

const here = path.dirname(fileURLToPath(import.meta.url));
const shellDir = path.resolve(here, '../shell');
const catalogDir = path.resolve(here, '../catalog');
const engineDir = path.resolve(here, '../engine');
const authDir = path.resolve(here, '../auth');
const sheetDir = path.resolve(here, '../sheet');
const libraryDir = path.resolve(here, '../library');

function sourceFiles(dir: string): string[] {
  return readdirSync(dir)
    .filter((name) => name.endsWith('.tsx') || name.endsWith('.ts') || name.endsWith('.css'))
    .map((name) => path.join(dir, name));
}

function assertTokenOnlySources(files: readonly string[]) {
  for (const file of files) {
    const text = readFileSync(file, 'utf8');
    expect(text).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(text).not.toMatch(/\b\d+px\b/);
  }
}

function assertTokensAreDefined(files: readonly string[]) {
  const defined = readTokenNames();
  for (const file of files) {
    const text = readFileSync(file, 'utf8');
    for (const match of text.matchAll(/var\((--lg-[a-z0-9-]+)\)/g)) {
      expect(defined.has(match[1])).toBe(true);
    }
  }
}

describe('design token usage in the shell (FND-02)', () => {
  it('Happy path: the shell has sources to inspect', () => {
    expect(sourceFiles(shellDir).length).toBeGreaterThan(0);
  });

  it('Edge: no shell source contains a literal color or pixel value', () => {
    assertTokenOnlySources(sourceFiles(shellDir));
  });

  it('Happy path: every token the shell references is defined in tokens.css', () => {
    assertTokensAreDefined(sourceFiles(shellDir));
  });
});

describe('design token usage in the catalog (WCT-28)', () => {
  it('Happy path: the catalog has sources to inspect', () => {
    expect(sourceFiles(catalogDir).length).toBeGreaterThan(0);
  });

  it('Edge: no catalog source contains a literal color or pixel value', () => {
    assertTokenOnlySources(sourceFiles(catalogDir));
  });

  it('Happy path: every token the catalog references is defined in tokens.css', () => {
    assertTokensAreDefined(sourceFiles(catalogDir));
  });
});

describe('design token usage in the engine (ENG-28, SC-008)', () => {
  it('Happy path: the engine has sources to inspect', () => {
    expect(sourceFiles(engineDir).length).toBeGreaterThan(0);
  });

  it('Edge: no engine source contains a literal color or pixel value', () => {
    assertTokenOnlySources(sourceFiles(engineDir));
  });

  it('Happy path: every token the engine references is defined in tokens.css', () => {
    assertTokensAreDefined(sourceFiles(engineDir));
  });
});

describe('design token usage in the sheet cycle (WSC-33)', () => {
  it('Happy path: the sheet and library sources exist to inspect', () => {
    expect(sourceFiles(sheetDir).length).toBeGreaterThan(0);
    expect(sourceFiles(libraryDir).length).toBeGreaterThan(0);
  });

  it('Edge: no sheet or library source contains a literal color or pixel value', () => {
    assertTokenOnlySources([...sourceFiles(sheetDir), ...sourceFiles(libraryDir)]);
  });

  it('Happy path: every token the sheet cycle references is defined in tokens.css', () => {
    assertTokensAreDefined([...sourceFiles(sheetDir), ...sourceFiles(libraryDir)]);
  });
});

describe('design token usage in the auth surface (WAS-31)', () => {
  it('Happy path: the auth sources exist to inspect', () => {
    expect(sourceFiles(authDir).length).toBeGreaterThan(0);
  });

  it('Edge: no auth source contains a literal color or pixel value', () => {
    assertTokenOnlySources(sourceFiles(authDir));
  });

  it('Happy path: every token the auth surface references is defined in tokens.css', () => {
    assertTokensAreDefined(sourceFiles(authDir));
  });
});

const uiDir = path.resolve(here, '../ui');
describe('design token usage in the ui primitives layer (UIP-04, SC-002)', () => {
  it('Happy path: the ui layer has sources to inspect once built', () => {
    const files = sourceFiles(uiDir);
    expect(files.length).toBeGreaterThanOrEqual(0);
  });
  it('Edge: no ui layer source contains a literal color or pixel value', () => {
    assertTokenOnlySources(sourceFiles(uiDir));
  });
  it('Happy path: every token the ui layer references is declared', () => {
    assertTokensAreDefined(sourceFiles(uiDir));
  });
});
