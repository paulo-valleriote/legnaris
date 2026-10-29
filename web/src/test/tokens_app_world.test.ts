import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { contrastRatio } from './contrast';
import { readThemeTokenNames, readThemeTokenValues, tokensCss } from './tokens_files';

const APP_SELECTOR = ':root:has(.lg-shell)';
const TOKEN_DECLARATION = /(--lg-[a-z0-9-]+)\s*:\s*([^;]+);/g;

function readAppWorldBlock(): Map<string, string> {
  const css = readFileSync(tokensCss, 'utf8');
  const start = css.indexOf(APP_SELECTOR);
  if (start === -1) {
    return new Map();
  }
  const open = css.indexOf('{', start);
  const close = css.indexOf('}', open);
  const values = new Map<string, string>();
  for (const match of css.slice(open + 1, close).matchAll(TOKEN_DECLARATION)) {
    values.set(match[1], match[2].trim());
  }
  return values;
}

function resolveAppWorld(): Map<string, string> {
  const declarations = new Map(readThemeTokenValues('light'));
  for (const [name, value] of readAppWorldBlock()) {
    declarations.set(name, value);
  }
  const resolve = (value: string, seen: Set<string>): string => {
    const alias = value.match(/^var\((--lg-[a-z0-9-]+)\)$/);
    const target = alias ? declarations.get(alias[1]) : undefined;
    if (!alias || target === undefined || seen.has(alias[1])) {
      return value;
    }
    return resolve(target, new Set([...seen, alias[1]]));
  };
  return new Map([...declarations].map(([name, value]) => [name, resolve(value, new Set())]));
}

const textPairs: readonly (readonly [string, string, number])[] = [
  ['--lg-color-text', '--lg-color-bg', 4.5],
  ['--lg-color-text', '--lg-color-surface', 4.5],
  ['--lg-color-text', '--lg-color-raised', 4.5],
  ['--lg-color-text-muted', '--lg-color-bg', 4.5],
  ['--lg-color-text-muted', '--lg-color-surface', 4.5],
  ['--lg-color-text-muted', '--lg-color-raised', 4.5],
  ['--lg-color-accent', '--lg-color-bg', 4.5],
  ['--lg-color-accent-text', '--lg-color-accent', 4.5],
  ['--lg-color-accent-text', '--lg-color-accent-hover', 4.5],
  ['--lg-color-danger', '--lg-color-surface', 4.5],
  ['--lg-color-danger-text', '--lg-color-danger', 4.5],
  ['--lg-color-focus', '--lg-color-bg', 3],
  ['--lg-color-focus', '--lg-color-surface', 3],
];

describe('the signed-in app world remaps the semantic layer onto the stage roles', () => {
  it('Edge: the remap block exists and declares dark color-scheme', () => {
    const block = readAppWorldBlock();

    expect(block.size).toBeGreaterThan(0);
    expect(readFileSync(tokensCss, 'utf8')).toMatch(
      /:root:has\(\.lg-shell\)\s*{\s*color-scheme: dark;/,
    );
  });

  it('Edge: every colour semantic the light theme declares is remapped, so no parchment leaks through', () => {
    const remapped = new Set(readAppWorldBlock().keys());
    const lightColours = [...readThemeTokenNames('light')].filter(
      (name) => name.startsWith('--lg-color-') && !name.startsWith('--lg-color-stage'),
    );

    expect(lightColours.filter((name) => !remapped.has(name))).toEqual([]);
  });

  it('Edge: the remapped values resolve to concrete colours', () => {
    const resolved = resolveAppWorld();

    for (const [name] of readAppWorldBlock()) {
      if (name.startsWith('--lg-color-')) {
        expect(resolved.get(name)).toMatch(/^(#|rgba?\()/);
      }
    }
  });

  for (const [foreground, background, minRatio] of textPairs) {
    it(`Happy path: ${foreground} on ${background} meets ${minRatio}:1 in the app world`, () => {
      const resolved = resolveAppWorld();

      expect(
        contrastRatio(resolved.get(foreground) as string, resolved.get(background) as string),
      ).toBeGreaterThanOrEqual(minRatio);
    });
  }
});
