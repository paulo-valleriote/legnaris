import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { DesignToken } from '../theme/tokens';

const here = path.dirname(fileURLToPath(import.meta.url));

export const tokensCss = path.resolve(here, '../theme/tokens.css');

/** Reads `tokens.css` into a `--lg-*` -> value map. */
export function readTokenValues(): Map<string, string> {
  const css = readFileSync(tokensCss, 'utf8');
  const values = new Map<string, string>();
  for (const match of css.matchAll(/(--lg-[a-z0-9-]+)\s*:\s*([^;]+);/g)) {
    values.set(match[1], match[2].trim());
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
