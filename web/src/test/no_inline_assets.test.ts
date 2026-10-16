import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const html = readFileSync(path.resolve(here, '../../index.html'), 'utf8');

describe('served HTML asset policy (FND-10)', () => {
  it('Happy path: the entry script is an external, same-origin module', () => {
    const scripts = [...html.matchAll(/<script\b[^>]*>/gi)].map((m) => m[0]);
    expect(scripts).toHaveLength(1);
    expect(scripts[0]).toContain('type="module"');
    expect(scripts[0]).toContain('/src/main.tsx');
  });

  it('Edge: there is no inline script body', () => {
    const bodies = [...html.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/gi)].map((m) => m[1]);
    for (const body of bodies) {
      expect(body.trim()).toBe('');
    }
  });

  it('Edge: no asset is loaded from a third-party origin', () => {
    const urls = [...html.matchAll(/\b(?:src|href)\s*=\s*"([^"]+)"/gi)].map((m) => m[1]);
    expect(urls.length).toBeGreaterThan(0);
    for (const url of urls) {
      expect(url.startsWith('/') || url.startsWith('data:')).toBe(true);
    }
  });
});
