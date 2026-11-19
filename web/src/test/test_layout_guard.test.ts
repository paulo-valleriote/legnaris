import { readdirSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const srcDir = path.resolve(here, '..');
const testDir = path.resolve(here);

const TEST_FILE = /\.test\.tsx?$/;

function testFilesOutside(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = path.join(dir, entry);

    if (statSync(full).isDirectory()) {
      return full === testDir ? [] : testFilesOutside(full);
    }

    return TEST_FILE.test(entry) ? [full] : [];
  });
}

describe('suite layout (LGN-270)', () => {
  it('Happy path: every test file lives flat in src/test/', () => {
    expect(testFilesOutside(srcDir)).toEqual([]);
  });

  it('Edge: the guard recognises both test suffixes', () => {
    expect(TEST_FILE.test('api_client.test.ts')).toBe(true);
    expect(TEST_FILE.test('auth_screen.test.tsx')).toBe(true);
    expect(TEST_FILE.test('sw.ts')).toBe(false);
    expect(TEST_FILE.test('theme.spec.ts')).toBe(false);
  });
});
