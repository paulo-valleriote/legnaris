import { existsSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * The landing photograph ships self-hosted, in sizes a phone does not have to over-download:
 * every address the page's `srcset` names must exist under `public/`, stay on the app's own
 * origin and stay light enough for a hero image.
 */
const here = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.resolve(here, '../../public');
const screenSource = readFileSync(path.resolve(here, '../landing/landing_screen.tsx'), 'utf8');

const HERO_IMAGES = [...screenSource.matchAll(/'(\/images\/hero-lava-\d+\.webp)(?: \d+w)?'/g)].map(
  (match) => match[1],
);
const MAX_HERO_BYTES = 200 * 1024;

describe('landing hero photograph', () => {
  it('Happy path: the page declares at least a small and a large size', () => {
    expect(new Set(HERO_IMAGES).size).toBeGreaterThanOrEqual(2);
  });

  it('Edge: every declared size exists in public/ and is a same-origin path', () => {
    for (const image of HERO_IMAGES) {
      expect(image.startsWith('/')).toBe(true);
      expect(image.startsWith('//')).toBe(false);
      expect(existsSync(path.join(publicDir, image))).toBe(true);
    }
  });

  it('Edge: no size exceeds the hero byte budget', () => {
    for (const image of HERO_IMAGES) {
      expect(statSync(path.join(publicDir, image)).size).toBeLessThanOrEqual(MAX_HERO_BYTES);
    }
  });

  it('Edge: the image is decorative and never requested from a third-party origin', () => {
    expect(screenSource).not.toMatch(/https?:\/\//);
  });
});
