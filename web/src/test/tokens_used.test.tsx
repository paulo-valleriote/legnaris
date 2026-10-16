import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { readTokenNames } from './tokens_files';

const here = path.dirname(fileURLToPath(import.meta.url));
const shellDir = path.resolve(here, '../shell');
const catalogDir = path.resolve(here, '../catalog');

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
