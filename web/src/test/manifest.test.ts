import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const manifest = JSON.parse(
  readFileSync(path.resolve(here, '../../public/manifest.webmanifest'), 'utf8'),
) as {
  name: string;
  short_name: string;
  start_url: string;
  scope: string;
  display: string;
  icons: { src: string; sizes: string; purpose: string }[];
};

describe('web app manifest (FND-06)', () => {
  it('Happy path: declares the installability identity fields', () => {
    expect(manifest.name).toBe('Legnaris');
    expect(manifest.short_name).toBe('Legnaris');
    expect(manifest.display).toBe('standalone');
    expect(manifest.start_url).toBe('/');
    expect(manifest.scope).toBe('/');
  });

  it('Edge: start_url stays inside scope', () => {
    expect(manifest.start_url.startsWith(manifest.scope)).toBe(true);
  });

  it('Happy path: ships 192 and 512 px icons in standard and maskable form', () => {
    const standard192 = manifest.icons.find((i) => i.sizes === '192x192' && i.purpose === 'any');
    const standard512 = manifest.icons.find((i) => i.sizes === '512x512' && i.purpose === 'any');
    const maskable192 = manifest.icons.find(
      (i) => i.sizes === '192x192' && i.purpose === 'maskable',
    );
    const maskable512 = manifest.icons.find(
      (i) => i.sizes === '512x512' && i.purpose === 'maskable',
    );

    expect(standard192).toBeDefined();
    expect(standard512).toBeDefined();
    expect(maskable192).toBeDefined();
    expect(maskable512).toBeDefined();
  });

  it('Edge: every icon source is same-origin', () => {
    for (const icon of manifest.icons) {
      expect(icon.src.startsWith('/')).toBe(true);
    }
  });
});
