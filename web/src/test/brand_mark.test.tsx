import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BrandMark } from '../shell/brand_mark';
import { BRAND_CORE_POINTS, BRAND_GLYPH_POINTS } from '../shell/brand_mark_geometry';

const here = path.dirname(fileURLToPath(import.meta.url));

function renderMark(): HTMLElement {
  const { container } = render(<BrandMark />);
  const svg = container.querySelector('svg');
  if (!svg) {
    throw new Error('BrandMark rendered no svg');
  }
  return svg as unknown as HTMLElement;
}

describe('BrandMark sigil', () => {
  it('Happy path: draws the diamond frame, the L glyph and one accent core', () => {
    const svg = renderMark();
    const polygons = [...svg.querySelectorAll('polygon')];

    expect(polygons).toHaveLength(3);
    expect(polygons[0].getAttribute('stroke')).toBe('currentColor');
    expect(polygons[1].getAttribute('fill')).toBe('currentColor');
    expect(polygons[2].getAttribute('class')).toBe('lg-brand__core');
  });

  it('Happy path: the frame and the L follow the surrounding text colour, so light and dark themes both hold contrast', () => {
    const svg = renderMark();
    const inked = [...svg.querySelectorAll('polygon')].filter(
      (polygon) => polygon.getAttribute('class') !== 'lg-brand__core',
    );

    expect(inked.length).toBeGreaterThan(0);
    for (const polygon of inked) {
      const paint = polygon.getAttribute('stroke') ?? polygon.getAttribute('fill');
      expect(paint).toBe('currentColor');
    }
  });

  it('Edge: the mark is decorative, hidden from assistive technology and not focusable', () => {
    const svg = renderMark();

    expect(svg.getAttribute('aria-hidden')).toBe('true');
    expect(svg.getAttribute('focusable')).toBe('false');
    expect(svg.getAttribute('role')).toBeNull();
  });

  it('Edge: the mark carries no literal colour, so only theme tokens paint it', () => {
    const svg = renderMark();
    const markup = svg.outerHTML;

    expect(markup).not.toMatch(/#[0-9a-f]{3,8}\b/i);
    expect(markup).not.toMatch(/\b(rgb|hsl)a?\(/i);
  });

  it('Edge: the mark contains no text, so the wordmark text alone provides the accessible name', () => {
    const svg = renderMark();

    expect(svg.textContent).toBe('');
    expect(svg.querySelector('title')).toBeNull();
  });

  it('Edge: the mark is viewBox-scaled and declares no fixed pixel size', () => {
    const svg = renderMark();

    expect(svg.getAttribute('viewBox')).toBe('0 0 120 120');
    expect(svg.getAttribute('width')).toBeNull();
    expect(svg.getAttribute('height')).toBeNull();
  });

  it('Edge: the first polygon inherits fill none from the root svg instead of repeating it', () => {
    const svg = renderMark();

    expect(svg.getAttribute('fill')).toBe('none');
    expect(svg.querySelectorAll('polygon')[0].getAttribute('fill')).toBeNull();
  });

  it('Happy path: the L and the core take their points from the shared geometry', () => {
    const polygons = [...renderMark().querySelectorAll('polygon')];

    expect(polygons[1].getAttribute('points')).toBe(BRAND_GLYPH_POINTS);
    expect(polygons[2].getAttribute('points')).toBe(BRAND_CORE_POINTS);
  });
});

describe('BrandMark accent colour', () => {
  it('Happy path: the accent core paints the brand-orange token, not the amber accent', () => {
    const css = readFileSync(path.resolve(here, '../shell/brand_mark.css'), 'utf8');

    expect(css).toMatch(/\.lg-brand__core\s*\{\s*fill:\s*var\(--lg-color-stage-brand\);?\s*\}/);
    expect(css).not.toMatch(/--lg-color-accent\b/);
  });

  it('Edge: no screen overrides the brand core fill, so the mark is drawn once', () => {
    for (const file of ['../auth/auth.css', '../landing/landing.css']) {
      expect(readFileSync(path.resolve(here, file), 'utf8'), file).not.toMatch(/lg-brand__core/);
    }
  });
});
