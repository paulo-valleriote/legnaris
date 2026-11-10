import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { icoImageCount, pngSize, publicFile } from './icon_files';
import { resolveThemeTokenValues } from './tokens_files';

const here = path.dirname(fileURLToPath(import.meta.url));
const html = readFileSync(path.resolve(here, '../../index.html'), 'utf8');
const manifest = JSON.parse(
  readFileSync(path.resolve(here, '../../public/manifest.webmanifest'), 'utf8'),
) as { icons: { src: string }[] };

type HeadLink = { rel: string; href: string; sizes?: string; type?: string };
type HeadMeta = { name: string; content: string; media?: string };

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

function namedMetas(): HeadMeta[] {
  return [...html.matchAll(/<meta\b[^>]*>/gi)]
    .map((match) => parseAttributes(match[0]))
    .filter((attributes) => attributes.name !== undefined)
    .map((attributes) => ({
      name: attributes.name,
      content: attributes.content ?? '',
      media: attributes.media,
    }));
}

function iconLinks(): HeadLink[] {
  return headLinks().filter((link) => link.rel === 'icon' || link.rel === 'apple-touch-icon');
}

const stageGround = resolveThemeTokenValues('dark').get('--lg-color-stage');

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

  it('Edge: every icon link points at a declared file type (svg, ico or png)', () => {
    for (const link of iconLinks()) {
      expect(link.href).toMatch(/\.(svg|ico|png)$/);
    }
  });

  it('Edge: every icon path the page and the manifest reference exists on disk', () => {
    const referenced = [
      ...iconLinks().map((link) => link.href),
      ...manifest.icons.map((icon) => icon.src),
    ];
    expect(referenced.length).toBeGreaterThan(0);
    for (const url of referenced) {
      expect(existsSync(publicFile(url)), url).toBe(true);
    }
  });

  it('Happy path: the manifest link is declared', () => {
    expect(headLinks().find((link) => link.rel === 'manifest')?.href).toBe('/manifest.webmanifest');
  });
});

describe('served HTML theme colour', () => {
  it('Happy path: both theme-color entries paint the stage ground the shell uses', () => {
    const metas = namedMetas().filter((meta) => meta.name === 'theme-color');
    expect(metas).toHaveLength(2);
    for (const meta of metas) {
      expect(meta.content.toLowerCase(), meta.media).toBe(stageGround);
    }
  });

  it('Edge: the theme-color entries are split by colour scheme, not left unconditional', () => {
    const media = namedMetas()
      .filter((meta) => meta.name === 'theme-color')
      .map((meta) => meta.media);
    expect(media).toEqual(['(prefers-color-scheme: light)', '(prefers-color-scheme: dark)']);
  });
});
