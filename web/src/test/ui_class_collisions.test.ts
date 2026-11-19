import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const srcRoot = path.resolve(here, '..');
const uiCss = path.resolve(srcRoot, 'ui/ui.css');

function featureStylesheets(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = path.join(dir, entry);
    if (statSync(full).isDirectory()) {
      return entry === 'ui' || entry === 'test' || entry === 'node_modules'
        ? []
        : featureStylesheets(full);
    }
    return entry.endsWith('.css') ? [full] : [];
  });
}

function blockSelectors(css: string): Set<string> {
  const blocks = new Set<string>();
  for (const match of css.matchAll(/(?:^|[\s,}])\.(lg-[a-z0-9-]+?)(?=[\s,{:.[>]|$)/gm)) {
    const name = match[1];
    if (name !== undefined && !name.includes('__') && !name.includes('--')) {
      blocks.add(name);
    }
  }
  return blocks;
}

function definesTopLevel(css: string, block: string): boolean {
  return new RegExp(`(^|\\n)\\.${block}\\s*\\{`).test(css);
}

describe('shared primitive class names', () => {
  it('Edge: the block helper ignores elements and modifiers', () => {
    const blocks = blockSelectors('.lg-a {} .lg-a__b {} .lg-a--c {} .lg-d:hover {}');
    expect([...blocks].sort()).toEqual(['lg-a', 'lg-d']);
  });

  it('Edge: no feature stylesheet restyles a shared primitive block at the top level', () => {
    const shared = blockSelectors(readFileSync(uiCss, 'utf8'));
    const collisions = featureStylesheets(srcRoot).flatMap((file) => {
      const css = readFileSync(file, 'utf8');
      return [...shared]
        .filter((block) => definesTopLevel(css, block))
        .map((block) => `${path.relative(srcRoot, file)} -> .${block}`);
    });
    expect(collisions).toEqual([]);
  });

  it('Happy path: the shared Field wrapper uses its own block, not the engine cell class', () => {
    const css = readFileSync(uiCss, 'utf8');
    expect(definesTopLevel(css, 'lg-form-field')).toBe(true);
    expect(definesTopLevel(css, 'lg-field')).toBe(false);
  });
});
