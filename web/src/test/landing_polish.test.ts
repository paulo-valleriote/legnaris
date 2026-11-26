import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const css = readFileSync(path.resolve(here, '../landing/landing.css'), 'utf8');
const baseCss = readFileSync(path.resolve(here, '../theme/base.css'), 'utf8');
const tokensCss = readFileSync(path.resolve(here, '../theme/tokens.css'), 'utf8');

function blockFor(selector: string): string {
  const start = css.indexOf(selector);
  expect(start, `${selector} is declared`).toBeGreaterThanOrEqual(0);
  return css.slice(start, css.indexOf('}', start));
}

describe('landing polish contract', () => {
  it('Happy path: the page only animates through the one global reduced-motion rule', () => {
    expect(css).toMatch(/animation:/);
    expect(css).not.toContain('prefers-reduced-motion');
    expect(baseCss).toContain('@media (prefers-reduced-motion: reduce)');
    expect(baseCss).toMatch(/animation-duration:\s*0\.01ms/);
    expect(baseCss).toMatch(/transition-duration:\s*0\.01ms/);
  });

  it('Edge: the page forces a dark colour scheme and canvas whatever the theme', () => {
    expect(blockFor(':root:has(.lg-landing)')).toContain('color-scheme: dark');
    expect(blockFor('body:has(.lg-landing)')).toContain('background-color: var(--lg-color-stage)');
  });

  it('Edge: the top-bar action keeps the full touch target', () => {
    expect(blockFor('.lg-landing__bar .lg-landing__cta')).not.toContain('min-height');
  });

  it('Edge: the sticky CTA sits at the viewport foot and leaves room on wide screens', () => {
    expect(blockFor('.lg-landing__sticky')).toContain('position: fixed');
    const wide = css.slice(css.indexOf('@media (min-width: 40rem)'));
    expect(wide).toMatch(/\.lg-landing__sticky\s*\{[^}]*display:\s*none/);
  });

  it('Happy path: section titles sit one display step below the hero and the close', () => {
    expect(tokensCss).toMatch(/--lg-type-display-sm:\s*clamp\(/);
    expect(blockFor('.lg-landing__title {')).toContain('font-size: var(--lg-type-display-sm)');
    expect(blockFor('.lg-landing__close-title {')).toContain(
      'font-size: var(--lg-type-display-md)',
    );
    expect(blockFor('.lg-landing__lead {')).toContain('font-size: var(--lg-type-18)');
  });

  it('Edge: on a phone the photograph keeps the ruin in frame and fades into the stage', () => {
    const phone = css.slice(css.indexOf('@media (max-width: 39.99rem)'));
    const photo = phone.slice(phone.indexOf('.lg-landing__photo {'));
    const block = photo.slice(0, photo.indexOf('}'));
    expect(block).toContain('object-position: 90% 50%');
    expect(block).toContain('mask-image');
  });

  it('Edge: the moments skeleton reserves three cards with the real card shell', () => {
    expect(blockFor('.lg-landing__moment-shell {')).toContain('min-height');
    const wide = css.slice(css.indexOf('@media (min-width: 56rem)'));
    expect(wide).toMatch(
      /\.lg-landing__moments-skeleton\s*\{[^}]*grid-template-columns:\s*repeat\(3/,
    );
  });

  it('Happy path: the hero chain is a compact strip whose derived values sit side by side', () => {
    expect(blockFor('\n.lg-chain {')).toContain('align-self: end');
    expect(blockFor('.lg-chain__derived {')).toContain(
      'grid-template-columns: repeat(3, minmax(0, 1fr))',
    );
    expect(blockFor('.lg-chain__step {')).toContain('width: var(--lg-size-control)');
  });

  it('Edge: the chain formulas leave the strip but stay readable to assistive technology', () => {
    const formula = css.slice(css.lastIndexOf('.lg-chain__formula {'));
    const block = formula.slice(0, formula.indexOf('}'));
    expect(block).toContain('clip-path: inset(50%)');
    expect(block).not.toContain('display: none');
  });

  it('Edge: the mounted demo inherits the stage world through the landing remap', () => {
    expect(tokensCss).toMatch(/:root:has\(\.lg-shell\),\s*:root:has\(\.lg-landing\)/);
  });
});
