import { describe, expect, it } from 'vitest';
import { contrastPairs, designTokens } from '../theme/tokens';
import { readThemeTokenNames, resolveThemeTokenValues } from './tokens_files';
import { contrastRatio } from './contrast';

const LINK_PAIRS = contrastPairs.filter((pair) => pair.foregroundVar.startsWith('--lg-color-link'));

describe('accent text role for links (AA on parchment and charcoal)', () => {
  it('Happy path: the link roles are documented semantics', () => {
    const names = designTokens.map((token) => token.name);
    expect(names).toContain('--lg-color-link');
    expect(names).toContain('--lg-color-link-hover');
  });

  it('Edge: the link role is declared in both themes', () => {
    for (const theme of ['light', 'dark'] as const) {
      expect(readThemeTokenNames(theme).has('--lg-color-link')).toBe(true);
      expect(readThemeTokenNames(theme).has('--lg-color-link-hover')).toBe(true);
    }
  });

  it('Happy path: link on bg and on surface is declared as a 4.5:1 pair in both themes', () => {
    for (const theme of ['light', 'dark'] as const) {
      for (const background of ['--lg-color-bg', '--lg-color-surface']) {
        expect(
          LINK_PAIRS.some(
            (pair) =>
              pair.theme === theme &&
              pair.foregroundVar === '--lg-color-link' &&
              pair.backgroundVar === background &&
              pair.minRatio === 4.5,
          ),
        ).toBe(true);
      }
    }
  });

  it.each(LINK_PAIRS)('Happy path: $theme $description meets $minRatio:1', (pair) => {
    const values = resolveThemeTokenValues(pair.theme);
    expect(
      contrastRatio(
        values.get(pair.foregroundVar) as string,
        values.get(pair.backgroundVar) as string,
      ),
    ).toBeGreaterThanOrEqual(pair.minRatio);
  });
});
