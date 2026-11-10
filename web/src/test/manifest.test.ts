import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.resolve(here, '../../public');
const manifestText = readFileSync(path.join(publicDir, 'manifest.webmanifest'), 'utf8');
const html = readFileSync(path.resolve(here, '../../index.html'), 'utf8');

const manifest = JSON.parse(manifestText) as {
  name: string;
  short_name: string;
  start_url: string;
  scope: string;
  display: string;
  background_color: string;
  theme_color: string;
  icons: { src: string; sizes: string; type: string; purpose: string }[];
};

const BRAND_PALETTE = ['#0a1a1e', '#f2692a', '#f1ebe1'];

type HeadLink = { rel: string; href: string; sizes?: string; type?: string };

function parseAttributes(tag: string): Record<string, string> {
  const attributes: Record<string, string> = {};
  for (const match of tag.matchAll(/([a-zA-Z-]+)\s*=\s*"([^"]*)"/g)) {
    attributes[match[1].toLowerCase()] = match[2];
  }
  return attributes;
}

function headLinks(): HeadLink[] {
  return [...html.matchAll(/<link\b[^>]*>/gi)].map((match) => {
    const attributes = parseAttributes(match[0]);
    return {
      rel: attributes.rel ?? '',
      href: attributes.href ?? '',
      sizes: attributes.sizes,
      type: attributes.type,
    };
  });
}

function iconLinks(): HeadLink[] {
  return headLinks().filter((link) => link.rel === 'icon' || link.rel === 'apple-touch-icon');
}

function publicFile(url: string): string {
  return path.join(publicDir, url);
}

function pngSize(file: string): { width: number; height: number } | null {
  const bytes = readFileSync(file);
  const signature = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
  if (bytes.length < 24 || !signature.every((byte, index) => bytes[index] === byte)) {
    return null;
  }
  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

function icoImageCount(file: string): number {
  const bytes = readFileSync(file);
  if (bytes.length < 6 || bytes.readUInt16LE(0) !== 0 || bytes.readUInt16LE(2) !== 1) {
    return -1;
  }
  return bytes.readUInt16LE(4);
}

function declaredSize(sizes: string | undefined): number | null {
  const match = /^(\d+)x(\d+)$/.exec(sizes ?? '');
  return match ? Number(match[1]) : null;
}

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
      expect(icon.src.startsWith('//')).toBe(false);
    }
  });

  it('Edge: every manifest icon resolves to an existing file with the declared PNG dimensions', () => {
    expect(manifest.icons.length).toBeGreaterThan(0);
    for (const icon of manifest.icons) {
      const file = publicFile(icon.src);
      expect(existsSync(file), icon.src).toBe(true);
      expect(icon.type).toBe('image/png');
      const size = pngSize(file);
      expect(size, icon.src).not.toBeNull();
      const edge = declaredSize(icon.sizes);
      expect(edge, icon.src).not.toBeNull();
      expect(size).toEqual({ width: edge, height: edge });
    }
  });

  it('Edge: the maskable icons are distinct files from the standard icons', () => {
    const maskable = manifest.icons.filter((i) => i.purpose === 'maskable').map((i) => i.src);
    const standard = manifest.icons.filter((i) => i.purpose === 'any').map((i) => i.src);
    for (const src of maskable) {
      expect(standard).not.toContain(src);
    }
  });

  it('Happy path: theme and background colours come from the new brand palette', () => {
    expect(BRAND_PALETTE).toContain(manifest.background_color.toLowerCase());
    expect(BRAND_PALETTE).toContain(manifest.theme_color.toLowerCase());
  });
});

describe('served HTML icon links', () => {
  it('Happy path: declares an SVG favicon', () => {
    const svg = iconLinks().find((link) => link.type === 'image/svg+xml');
    expect(svg).toBeDefined();
    expect(svg?.rel).toBe('icon');
    expect(svg?.href.endsWith('.svg')).toBe(true);
    expect(readFileSync(publicFile(svg!.href), 'utf8')).toMatch(/^<svg\b/);
  });

  it('Happy path: declares ICO and 16, 32 and 48 px PNG fallbacks', () => {
    const ico = iconLinks().find((link) => link.href.endsWith('.ico'));
    expect(ico).toBeDefined();
    expect(icoImageCount(publicFile(ico!.href))).toBeGreaterThanOrEqual(2);

    for (const edge of [16, 32, 48]) {
      const png = iconLinks().find(
        (link) => link.type === 'image/png' && link.sizes === `${edge}x${edge}`,
      );
      expect(png, `${edge}px`).toBeDefined();
      expect(pngSize(publicFile(png!.href))).toEqual({ width: edge, height: edge });
    }
  });

  it('Happy path: the apple-touch icon is the 180 px dedicated file', () => {
    const apple = iconLinks().find((link) => link.rel === 'apple-touch-icon');
    expect(apple).toBeDefined();
    expect(apple?.href).toMatch(/apple-touch-icon\.png$/);
    expect(pngSize(publicFile(apple!.href))).toEqual({ width: 180, height: 180 });
  });

  it('Edge: every icon link is same-origin and resolves to an existing file', () => {
    expect(iconLinks().length).toBeGreaterThan(0);
    for (const link of iconLinks()) {
      expect(link.href.startsWith('/'), link.href).toBe(true);
      expect(link.href.startsWith('//'), link.href).toBe(false);
      expect(existsSync(publicFile(link.href)), link.href).toBe(true);
    }
  });

  it('Edge: every icon link stays on a declared file type (no stale or foreign references)', () => {
    for (const link of iconLinks()) {
      expect(link.href).toMatch(/\.(svg|ico|png)$/);
    }
  });

  it('Edge: the manifest link is still declared', () => {
    expect(headLinks().find((link) => link.rel === 'manifest')?.href).toBe('/manifest.webmanifest');
  });

  it('Edge: no icon file referenced by the page or manifest was removed', () => {
    const referenced = [
      ...manifest.icons.map((icon) => icon.src),
      ...iconLinks().map((link) => link.href),
    ];
    for (const url of referenced) {
      expect(existsSync(publicFile(url)), url).toBe(true);
    }
  });

  it('Edge: the SVG favicon carries no script or external reference', () => {
    const svg = readFileSync(publicFile('/icons/favicon.svg'), 'utf8');
    expect(svg).not.toMatch(/<script/i);
    expect(svg).not.toMatch(/\son[a-z]+\s*=/i);
    expect(svg).not.toMatch(/(href|src)\s*=\s*"(?!#)/i);
  });
});
