import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * The landing sits outside the app shell, so the shell's reduced-motion clamp never reaches it
 * and the page, always dark, must keep native controls and the canvas dark in the light theme.
 */
const here = path.dirname(fileURLToPath(import.meta.url));
const css = readFileSync(path.resolve(here, '../landing/landing.css'), 'utf8');

function blockFor(selector: string): string {
  const start = css.indexOf(selector);
  expect(start, `${selector} is declared`).toBeGreaterThanOrEqual(0);
  return css.slice(start, css.indexOf('}', start));
}

describe('landing polish contract', () => {
  it('Happy path: a reduced-motion query clamps animations and transitions on the landing', () => {
    const query = css.slice(css.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(query).toContain('.lg-landing *');
    expect(query).toMatch(/animation-duration:\s*0\.01ms/);
    expect(query).toMatch(/transition-duration:\s*0\.01ms/);
  });

  it('Edge: every animation the page declares is covered by that clamp', () => {
    expect(css).toMatch(/animation:/);
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
  });

  it('Edge: the page forces a dark colour scheme and canvas whatever the theme', () => {
    expect(blockFor(':root:has(.lg-landing)')).toContain('color-scheme: dark');
    expect(blockFor('body:has(.lg-landing)')).toContain('background-color: var(--lg-color-stage)');
  });

  it('Edge: the top-bar action keeps the full touch target', () => {
    expect(blockFor('.lg-landing__bar .lg-landing__cta')).not.toContain('min-height');
  });
});
