import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const conf = readFileSync(path.resolve(here, '../../nginx.conf'), 'utf8');

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/** The body of the `location <selector> { ... }` block, or null when the config has none. */
function locationBlock(selector: string): string | null {
  const match = new RegExp(`location\\s+${escapeRegExp(selector)}\\s*\\{([^}]*)\\}`).exec(conf);
  return match ? match[1] : null;
}

describe('nginx caching rules for the static host', () => {
  it('Happy path: /icons/ is served with no-cache, since its file names are not content-hashed', () => {
    const block = locationBlock('/icons/');
    expect(block).not.toBeNull();
    expect(block).toMatch(/add_header\s+Cache-Control\s+"no-cache"\s*;/);
    expect(block).not.toMatch(/immutable|max-age/);
  });

  it('Edge: a missing /icons/ file is a 404, never the SPA shell', () => {
    expect(locationBlock('/icons/')).toMatch(/try_files\s+\$uri\s+=404\s*;/);
  });

  it('Edge: /icons/ is not caught by the immutable /assets/ rule', () => {
    expect(locationBlock('/assets/')).toMatch(/max-age=31536000, immutable/);
    expect(locationBlock('/icons/')).not.toMatch(/immutable/);
  });

  it('Happy path: the existing rules keep their caching (hashed assets immutable, shell and manifest no-cache)', () => {
    expect(locationBlock('/assets/')).toMatch(/max-age=31536000, immutable/);
    for (const selector of [
      '= /index.html',
      '= /manifest.webmanifest',
      '= /sw.js',
      '= /config.json',
    ]) {
      expect(locationBlock(selector), selector).toMatch(/Cache-Control\s+"no-cache"/);
    }
  });
});
