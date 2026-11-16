import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { DesignToken } from '../theme/tokens';

const here = path.dirname(fileURLToPath(import.meta.url));

export const tokensCss = path.resolve(here, '../theme/tokens.css');

export type ThemeName = 'light' | 'dark';

export const DARK_THEME_SELECTOR = "[data-theme='dark']";

const TOKEN_DECLARATION = /(--lg-[a-z0-9-]+)\s*:\s*([^;]+);/g;

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

export function readThemeBlocks(): Record<ThemeName, string> {
  const css = readFileSync(tokensCss, 'utf8');
  const darkStart = css.indexOf(DARK_THEME_SELECTOR);
  const lightSource = darkStart === -1 ? css : css.slice(0, darkStart);
  return {
    light: blockBody(lightSource, 0),
    dark: darkStart === -1 ? '' : blockBody(css, darkStart),
  };
}

export function readThemeTokenValues(theme: ThemeName): Map<string, string> {
  const values = new Map<string, string>();
  for (const match of readThemeBlocks()[theme].matchAll(TOKEN_DECLARATION)) {
    values.set(match[1], match[2].trim());
  }
  return values;
}

export function readThemeTokenNames(theme: ThemeName): Set<string> {
  return new Set(readThemeTokenValues(theme).keys());
}

export function resolveThemeTokenValues(theme: ThemeName): Map<string, string> {
  const declarations = new Map(readThemeTokenValues('light'));
  for (const [name, value] of readThemeTokenValues(theme)) {
    declarations.set(name, value);
  }

  const resolved = new Map<string, string>();
  for (const [name, value] of declarations) {
    resolved.set(name, resolveAlias(value, declarations, new Set([name])));
  }
  return resolved;
}

function resolveAlias(value: string, declarations: Map<string, string>, seen: Set<string>): string {
  const alias = value.trim().match(/^var\((--lg-[a-z0-9-]+)\)$/);
  if (!alias || seen.has(alias[1])) {
    return value.trim();
  }
  const target = declarations.get(alias[1]);
  if (target === undefined) {
    return value.trim();
  }
  seen.add(alias[1]);
  return resolveAlias(target, declarations, seen);
}

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

export function declaredTokenNames(tokens: readonly DesignToken[]): string[] {
  return tokens.map((token) => token.name);
}

export function namesPresentInOneOnly(
  first: ReadonlySet<string>,
  second: ReadonlySet<string>,
): string[] {
  const union = new Set([...first, ...second]);
  return [...union].filter((name) => first.has(name) !== second.has(name)).sort();
}

export function unresolvedPairVars(
  pairs: readonly { readonly foregroundVar: string; readonly backgroundVar: string }[],
  declared: ReadonlySet<string>,
): string[] {
  const missing = new Set<string>();
  for (const pair of pairs) {
    if (!declared.has(pair.foregroundVar)) {
      missing.add(pair.foregroundVar);
    }
    if (!declared.has(pair.backgroundVar)) {
      missing.add(pair.backgroundVar);
    }
  }
  return [...missing].sort();
}
