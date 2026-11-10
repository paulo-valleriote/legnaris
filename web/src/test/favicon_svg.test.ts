import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { BRAND_CORE_POINTS, BRAND_GLYPH_POINTS } from '../shell/brand_mark_geometry';
import { publicFile } from './icon_files';
import { resolveThemeTokenValues } from './tokens_files';

const svg = readFileSync(publicFile('/icons/favicon.svg'), 'utf8');
const brandOrange = resolveThemeTokenValues('dark').get('--lg-color-stage-brand');
const pointSets = [...svg.matchAll(/points="([^"]+)"/g)].map((match) => match[1]);

// The favicon draws its frame as a solid brand-orange diamond instead of BrandMark's stroked
// outline, because a favicon has no surrounding text colour to inherit. Only the L and the core
// are shared, so those two point sets must match BrandMark exactly.
describe('favicon.svg geometry', () => {
  it('Happy path: the L and the accent core use the same points as BrandMark', () => {
    expect(pointSets).toContain(BRAND_GLYPH_POINTS);
    expect(pointSets).toContain(BRAND_CORE_POINTS);
  });

  it('Happy path: the solid frame is the brand-orange token', () => {
    expect(brandOrange).toBe('#f2692a');
    expect(svg.toLowerCase()).toContain(`fill="${brandOrange}"`);
  });

  it('Edge: the SVG favicon carries no script, event handler or external reference', () => {
    expect(svg).toMatch(/^<svg\b/);
    expect(svg).not.toMatch(/<script/i);
    expect(svg).not.toMatch(/\son[a-z]+\s*=/i);
    expect(svg).not.toMatch(/(href|src)\s*=\s*"(?!#)/i);
  });
});
