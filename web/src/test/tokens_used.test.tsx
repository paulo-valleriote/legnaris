import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { readTokenNames } from './tokens_files';

const here = path.dirname(fileURLToPath(import.meta.url));
const shellDir = path.resolve(here, '../shell');

function shellFiles(): string[] {
  return readdirSync(shellDir)
    .filter((name) => name.endsWith('.tsx') || name.endsWith('.ts') || name.endsWith('.css'))
    .map((name) => path.join(shellDir, name));
}

describe('design token usage in the shell (FND-02)', () => {
  it('Happy path: the shell has sources to inspect', () => {
    expect(shellFiles().length).toBeGreaterThan(0);
  });

  it('Edge: no shell source contains a literal color or pixel value', () => {
    for (const file of shellFiles()) {
      const text = readFileSync(file, 'utf8');
      expect(text).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
      expect(text).not.toMatch(/\b\d+px\b/);
    }
  });

  it('Happy path: every token the shell references is defined in tokens.css', () => {
    const defined = readTokenNames();
    for (const file of shellFiles()) {
      const text = readFileSync(file, 'utf8');
      for (const match of text.matchAll(/var\((--lg-[a-z0-9-]+)\)/g)) {
        expect(defined.has(match[1])).toBe(true);
      }
    }
  });
});
