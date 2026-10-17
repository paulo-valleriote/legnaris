import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const html = readFileSync(path.resolve(here, '../../index.html'), 'utf8');

function scriptTags(): string[] {
  return [...html.matchAll(/<script\b[^>]*>/gi)].map((match) => match[0]);
}

describe('served HTML asset policy (FND-10)', () => {
  it('Happy path: the entry script is an external, same-origin module', () => {
    const entry = scriptTags().find((tag) => tag.includes('type="module"'));
    expect(entry).toBeDefined();
    expect(entry).toContain('/src/main.tsx');
  });

  it('Happy path: the theme boot script is an external, same-origin classic script (UIF-20)', () => {
    const boot = scriptTags().find((tag) => tag.includes('theme-init.js'));
    expect(boot).toBeDefined();
    expect(boot).not.toContain('type="module"');
    expect(boot).toContain('src="/theme-init.js"');
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
