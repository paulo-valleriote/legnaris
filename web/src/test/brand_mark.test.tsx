import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BrandMark } from '../shell/brand_mark';

function renderMark(): HTMLElement {
  const { container } = render(<BrandMark />);
  const svg = container.querySelector('svg');
  if (!svg) {
    throw new Error('BrandMark rendered no svg');
  }
  return svg as unknown as HTMLElement;
}

describe('BrandMark sigil', () => {
  it('Happy path: draws the diamond frame, the L glyph and one orange accent core', () => {
    const svg = renderMark();
    const polygons = [...svg.querySelectorAll('polygon')];

    expect(polygons).toHaveLength(3);
    expect(polygons[0].getAttribute('fill')).toBe('none');
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
});
