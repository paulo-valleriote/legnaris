import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { declaredSize, pngSize, publicFile } from './icon_files';
import { resolveThemeTokenValues } from './tokens_files';

const here = path.dirname(fileURLToPath(import.meta.url));
const manifest = JSON.parse(
  readFileSync(path.resolve(here, '../../public/manifest.webmanifest'), 'utf8'),
) as {
  name: string;
  short_name: string;
  start_url: string;
  scope: string;
  display: string;
  background_color: string;
  theme_color: string;
  icons: { src: string; sizes: string; type: string; purpose: string }[];
};

// The shell paints the browser chrome in the stage ground (DESIGN.md, "Browser chrome"), so the
// manifest colours resolve from the same token rather than a copied hex.
const stageGround = resolveThemeTokenValues('dark').get('--lg-color-stage');

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
    const find = (sizes: string, purpose: string) =>
      manifest.icons.find((icon) => icon.sizes === sizes && icon.purpose === purpose);

    expect(find('192x192', 'any')).toBeDefined();
    expect(find('512x512', 'any')).toBeDefined();
    expect(find('192x192', 'maskable')).toBeDefined();
    expect(find('512x512', 'maskable')).toBeDefined();
  });

  it('Edge: every icon source is same-origin', () => {
    for (const icon of manifest.icons) {
      expect(icon.src.startsWith('/'), icon.src).toBe(true);
      expect(icon.src.startsWith('//'), icon.src).toBe(false);
    }
  });

  it('Edge: every manifest icon is a PNG whose pixels match its declared square size', () => {
    expect(manifest.icons.length).toBeGreaterThan(0);
    for (const icon of manifest.icons) {
      const edge = declaredSize(icon.sizes);
      expect(edge, icon.src).not.toBeNull();
      expect(icon.type, icon.src).toBe('image/png');
      expect(pngSize(publicFile(icon.src)), icon.src).toEqual({ width: edge, height: edge });
    }
  });

  it('Edge: the maskable icons are distinct files from the standard icons', () => {
    const maskable = manifest.icons.filter((icon) => icon.purpose === 'maskable').map((i) => i.src);
    const standard = manifest.icons.filter((icon) => icon.purpose === 'any').map((i) => i.src);
    for (const src of maskable) {
      expect(standard).not.toContain(src);
    }
  });

  it('Happy path: theme_color is the stage ground token, the colour the shell paints in browser chrome', () => {
    expect(stageGround).toBe('#061516');
    expect(manifest.theme_color.toLowerCase()).toBe(stageGround);
  });

  it('Happy path: background_color is the stage ground token, since the teal #0A1A1E is not a token', () => {
    expect(manifest.background_color.toLowerCase()).toBe(stageGround);
  });
});
