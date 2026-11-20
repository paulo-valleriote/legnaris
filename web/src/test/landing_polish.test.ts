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

  it('Edge: the mounted demo inherits the stage world through the landing remap', () => {
    expect(tokensCss).toMatch(/:root:has\(\.lg-shell\),\s*:root:has\(\.lg-landing\)/);
  });
});
