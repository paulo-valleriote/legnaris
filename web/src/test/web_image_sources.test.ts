import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import vendoredTemplate from '../landing/fifth_character_template.json';
import goldenTemplate from '../../../testdata/templates/fifth_character_v1.json';

const webRoot = resolve(__dirname, '../..');
const srcRoot = join(webRoot, 'src');
const importPattern =
  /(?:import|export)[^'"]*?from\s*['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)/g;

function productionSources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      return name === 'test' ? [] : productionSources(path);
    }
    return /\.(ts|tsx)$/.test(name) && !/\.test\.tsx?$/.test(name) ? [path] : [];
  });
}

function escapingImports(file: string): string[] {
  const text = readFileSync(file, 'utf8');
  return [...text.matchAll(importPattern)]
    .map((match) => match[1] ?? match[2])
    .filter(
      (specifier): specifier is string => specifier !== undefined && specifier.startsWith('.'),
    )
    .filter((specifier) => relative(webRoot, resolve(file, '..', specifier)).startsWith('..'));
}

describe('web image build sources', () => {
  it('Edge: no production source imports a file outside web/, the Docker build context', () => {
    const offenders = productionSources(srcRoot).flatMap((file) =>
      escapingImports(file).map((specifier) => `${relative(webRoot, file)} -> ${specifier}`),
    );

    expect(offenders).toEqual([]);
  });

  it('Happy path: the vendored 5E Character template matches the backend golden', () => {
    expect(vendoredTemplate).toEqual(goldenTemplate);
  });
});
