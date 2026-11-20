import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { contrastPairs, designTokens } from '../theme/tokens';
import type { ThemeName } from './tokens_files';
import {
  namesPresentInOneOnly,
  readThemeTokenNames,
  resolveThemeTokenValues,
  unresolvedPairVars,
} from './tokens_files';
import { contrastRatio } from './contrast';

const here = path.dirname(fileURLToPath(import.meta.url));
const baseCss = readFileSync(path.resolve(here, '../theme/base.css'), 'utf8');

const THEMES: readonly ThemeName[] = ['light', 'dark'];

const semanticTokenNames = designTokens
  .filter((token) => token.layer === 'semantic')
  .map((token) => token.name);

function declaredSemantics(theme: ThemeName): Set<string> {
  const declared = readThemeTokenNames(theme);
  return new Set(semanticTokenNames.filter((name) => declared.has(name)));
}

describe('design token contrast across both themes', () => {
  it('Edge: a token declared in one theme only is reported missing in the other', () => {
    expect(
      namesPresentInOneOnly(new Set(['--lg-only-light']), new Set(['--lg-only-dark'])),
    ).toEqual(['--lg-only-dark', '--lg-only-light']);
  });

  it('Edge: an unknown pair variable is reported when the theme does not declare it', () => {
    const declared = readThemeTokenNames('light');
    expect(
      unresolvedPairVars(
        [{ foregroundVar: '--lg-color-does-not-exist', backgroundVar: '--lg-color-bg' }],
        declared,
      ),
    ).toEqual(['--lg-color-does-not-exist']);
  });

  it('Edge: every semantic token resolves in both themes (no one-theme token)', () => {
    expect(namesPresentInOneOnly(declaredSemantics('light'), declaredSemantics('dark'))).toEqual(
      [],
    );
  });

  it('Edge: every contrast pair resolves its two variables in both themes', () => {
    for (const theme of THEMES) {
      expect(unresolvedPairVars(contrastPairs, readThemeTokenNames(theme))).toEqual([]);
    }
  });

  it('Edge: every documented design token resolves to a value in tokens.css', () => {
    const declared = readThemeTokenNames('light');
    for (const token of designTokens) {
      expect(declared.has(token.name)).toBe(true);
    }
  });

  for (const pair of contrastPairs) {
    it(`Happy path: ${pair.theme} ${pair.description} meets ${pair.minRatio}:1`, () => {
      const values = resolveThemeTokenValues(pair.theme);
      const foreground = values.get(pair.foregroundVar);
      const background = values.get(pair.backgroundVar);

      expect(foreground).toBeDefined();
      expect(background).toBeDefined();
      expect(contrastRatio(foreground as string, background as string)).toBeGreaterThanOrEqual(
        pair.minRatio,
      );
    });
  }
});

describe('sheet affordance contrast', () => {
  function declaredPair(foregroundVar: string, backgroundVar: string): boolean {
    return THEMES.every((theme) =>
      contrastPairs.some(
        (pair) =>
          pair.theme === theme &&
          pair.foregroundVar === foregroundVar &&
          pair.backgroundVar === backgroundVar,
      ),
    );
  }

  it('Edge: subtle text is asserted on the surface layer in both themes', () => {
    expect(declaredPair('--lg-color-text-subtle', '--lg-color-surface')).toBe(true);
  });

  it('Edge: the lava accent is asserted on the surface and raised layers in both themes', () => {
    expect(declaredPair('--lg-color-accent', '--lg-color-surface')).toBe(true);
    expect(declaredPair('--lg-color-accent', '--lg-color-raised')).toBe(true);
  });

  it('Edge: the rollable underline colour is asserted on both sheet layers in both themes', () => {
    expect(declaredPair('--lg-color-link', '--lg-color-surface')).toBe(true);
    expect(declaredPair('--lg-color-link', '--lg-color-raised')).toBe(true);
  });
});

describe('reduced motion', () => {
  it('Edge: a stylesheet with no reduced-motion block does not match the guard', () => {
    expect(/@media\s*\(prefers-reduced-motion:\s*reduce\)/.test('body { margin: 0; }')).toBe(false);
  });

  it('Happy path: base.css zeroes non-essential animation under prefers-reduced-motion', () => {
    const block = baseCss.match(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]*)\}/,
    )?.[1];
    expect(block).toBeDefined();
    expect(block).toMatch(/animation-duration:\s*0/);
    expect(block).toMatch(/transition-duration:\s*0/);
  });
});
