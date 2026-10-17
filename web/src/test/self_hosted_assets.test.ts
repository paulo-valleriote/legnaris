import { existsSync, readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * The self-hosted asset contract: the font and its license are committed and served
 * same-origin, the `@font-face` declares the variable axis and swap behaviour, and nothing
 * in the theme sources or the document head reaches a third-party origin (UIF-14, UIF-15,
 * UIF-16, SC-005).
 */
const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '../..');
const themeDir = path.resolve(here, '../theme');
const fontDir = path.resolve(webRoot, 'public/fonts/geist');

function read(relative: string): string {
  const file = path.resolve(webRoot, relative);
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const fontsCss = () => read('src/theme/fonts.css');
const tokensCss = () => read('src/theme/tokens.css');
const indexHtml = () => read('index.html');

/** Every `@font-face` block body in a stylesheet. */
function fontFaceBlocks(css: string): string[] {
  return [...css.matchAll(/@font-face\s*\{([\s\S]*?)\}/g)].map((match) => match[1]);
}

/** An asset address that leaves the app's own origin. */
function isThirdPartyUrl(url: string): boolean {
  return /^(?:https?:)?\/\//i.test(url.trim().replace(/^['"]|['"]$/g, ''));
}

/** Every third-party font address a stylesheet would request. */
function thirdPartyFontUrls(css: string): string[] {
  const urls: string[] = [];
  for (const block of fontFaceBlocks(css)) {
    for (const match of block.matchAll(/url\(\s*(['"]?)([^'")]+)\1\s*\)/g)) {
      if (isThirdPartyUrl(match[2])) {
        urls.push(match[2]);
      }
    }
  }
  return urls;
}

/** Whether a font directory carries an open-font license beside the artifact. */
function hasCommittedLicense(dir: string): boolean {
  if (!existsSync(dir)) {
    return false;
  }
  return readdirSync(dir).some((name) => /license/i.test(name));
}

/** Every stylesheet the theme directory ships. */
function themeFiles(): string[] {
  if (!existsSync(themeDir)) {
    return [];
  }
  return readdirSync(themeDir)
    .filter((name) => name.endsWith('.css'))
    .map((name) => path.join(themeDir, name));
}

describe('self-hosted font assets (UIF-14, UIF-15, UIF-16, SC-005)', () => {
  it('Edge: a @font-face with a third-party url() is flagged', () => {
    const bad =
      "@font-face { font-family: 'Geist'; src: url('https://cdn.example.com/Geist.woff2') format('woff2'); }";
    expect(thirdPartyFontUrls(bad)).toEqual(['https://cdn.example.com/Geist.woff2']);
  });

  it('Edge: a font directory without a license is reported missing', () => {
    expect(hasCommittedLicense(path.resolve(here, '__no_such_font_dir__'))).toBe(false);
  });

  it('Edge: fonts.css, when absent, declares no face and no third-party origin', () => {
    expect(thirdPartyFontUrls(read('src/theme/__no_such_fonts.css'))).toEqual([]);
  });

  it('Happy path: the font artifact and its OFL license are committed beside each other', () => {
    expect(existsSync(path.join(fontDir, 'Geist-Variable.woff2'))).toBe(true);
    expect(hasCommittedLicense(fontDir)).toBe(true);
  });

  it('Happy path: fonts.css declares one self-hosted variable face with swap', () => {
    const blocks = fontFaceBlocks(fontsCss());
    expect(blocks).toHaveLength(1);
    expect(blocks[0]).toContain("font-family: 'Geist'");
    expect(blocks[0]).toContain("url('/fonts/geist/Geist-Variable.woff2')");
    expect(blocks[0]).toContain("format('woff2')");
    expect(blocks[0]).toMatch(/font-weight:\s*100\s+900/);
    expect(blocks[0]).toMatch(/font-display:\s*swap/);
    expect(thirdPartyFontUrls(fontsCss())).toEqual([]);
  });

  it('Happy path: index.html preloads the same-origin font for the first paint', () => {
    const preload = indexHtml().match(/<link[^>]*rel="preload"[^>]*>/i)?.[0] ?? '';
    expect(preload).toContain('as="font"');
    expect(preload).toContain('type="font/woff2"');
    expect(preload).toContain('crossorigin');
    expect(preload).toContain('/fonts/geist/Geist-Variable.woff2');
  });

  it('Happy path: --lg-font-family leads with the vendored family and keeps a system fallback', () => {
    const value =
      tokensCss()
        .match(/--lg-font-family\s*:\s*([^;]+);/)?.[1]
        ?.trim() ?? '';
    expect(value.startsWith("'Geist'")).toBe(true);
    expect(value).toContain('system-ui');
  });

  it('Edge: no theme source or index.html loads a third-party font, style or script', () => {
    for (const file of [...themeFiles(), path.resolve(webRoot, 'index.html')]) {
      const text = readFileSync(file, 'utf8');
      for (const match of text.matchAll(/\b(?:src|href)\s*=\s*"([^"]+)"/gi)) {
        expect(isThirdPartyUrl(match[1])).toBe(false);
      }
      for (const match of text.matchAll(/url\(\s*(['"]?)([^'")]+)\1\s*\)/g)) {
        expect(isThirdPartyUrl(match[2])).toBe(false);
      }
    }
  });
});
