import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { DesignToken } from '../theme/tokens';

const here = path.dirname(fileURLToPath(import.meta.url));

export const tokensCss = path.resolve(here, '../theme/tokens.css');

/** The two themes `tokens.css` declares: light under `:root`, dark under its selector. */
export type ThemeName = 'light' | 'dark';

/** The selector that opens the dark theme block. */
export const DARK_THEME_SELECTOR = "[data-theme='dark']";

const TOKEN_DECLARATION = /(--lg-[a-z0-9-]+)\s*:\s*([^;]+);/g;

/** The text inside the first balanced block that opens at or after `from`. */
function blockBody(css: string, from: number): string {
  const open = css.indexOf('{', from);
  if (open === -1) {
    return '';
  }
  let depth = 0;
  for (let index = open; index < css.length; index += 1) {
    const char = css[index];
    if (char === '{') {
      depth += 1;
    } else if (char === '}') {
      depth -= 1;
      if (depth === 0) {
        return css.slice(open + 1, index);
      }
    }
  }
  return css.slice(open + 1);
}

/**
 * Splits the token file into its light and dark blocks, so a theme can be read on its own.
 * The light block is the root declaration; the dark block is the one behind the dark selector
 * (`UIF-02`).
 */
export function readThemeBlocks(): Record<ThemeName, string> {
  const css = readFileSync(tokensCss, 'utf8');
  const darkStart = css.indexOf(DARK_THEME_SELECTOR);
  const lightSource = darkStart === -1 ? css : css.slice(0, darkStart);
  return {
    light: blockBody(lightSource, 0),
    dark: darkStart === -1 ? '' : blockBody(css, darkStart),
  };
}

/** Reads one theme's `--lg-*` declarations into a name -> value map. */
export function readThemeTokenValues(theme: ThemeName): Map<string, string> {
  const values = new Map<string, string>();
  for (const match of readThemeBlocks()[theme].matchAll(TOKEN_DECLARATION)) {
    values.set(match[1], match[2].trim());
  }
  return values;
}

/** The names a single theme declares. */
export function readThemeTokenNames(theme: ThemeName): Set<string> {
  return new Set(readThemeTokenValues(theme).keys());
}

/**
 * Reads `tokens.css` into a `--lg-*` -> value map over both themes; a name declared in both keeps
 * its light value, because only presence is asserted from this map.
 */
export function readTokenValues(): Map<string, string> {
  const values = readThemeTokenValues('dark');
  for (const [name, value] of readThemeTokenValues('light')) {
    values.set(name, value);
  }
  return values;
}

export function readTokenNames(): Set<string> {
  return new Set(readTokenValues().keys());
}

/** Every design token that must resolve to a real value in `tokens.css`. */
export function declaredTokenNames(tokens: readonly DesignToken[]): string[] {
  return tokens.map((token) => token.name);
}
