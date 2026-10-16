import { describe, expect, it } from 'vitest';
import { contrastPairs, designTokens } from '../theme/tokens';
import { readTokenNames, readTokenValues } from './tokens_files';
import { contrastRatio } from './contrast';

describe('design token contrast (FND-05)', () => {
  const values = readTokenValues();

  for (const pair of contrastPairs) {
    it(`Happy path: ${pair.description} meets ${pair.minRatio}:1`, () => {
      const foreground = values.get(pair.foregroundVar);
      const background = values.get(pair.backgroundVar);

      expect(foreground).toBeDefined();
      expect(background).toBeDefined();
      expect(contrastRatio(foreground as string, background as string)).toBeGreaterThanOrEqual(
        pair.minRatio,
      );
    });
  }

  it('Edge: every contrast pair references a declared color token', () => {
    const declared = readTokenNames();
    for (const pair of contrastPairs) {
      expect(declared.has(pair.foregroundVar)).toBe(true);
      expect(declared.has(pair.backgroundVar)).toBe(true);
    }
  });

  it('Edge: every documented design token resolves to a value in tokens.css', () => {
    const declared = readTokenNames();
    for (const token of designTokens) {
      expect(declared.has(token.name)).toBe(true);
    }
  });
});
