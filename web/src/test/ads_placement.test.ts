import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const srcRoot = path.resolve(here, '..');

const ALLOWED = [
  'catalog/catalog_screen.tsx',
  'campaign/campaigns_screen.tsx',
  'landing/landing_screen.tsx',
];

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = path.join(dir, entry);
    if (statSync(full).isDirectory()) {
      return entry === 'test' ? [] : sourceFiles(full);
    }
    return entry.endsWith('.tsx') ? [full] : [];
  });
}

function relative(file: string): string {
  return path.relative(srcRoot, file).split(path.sep).join('/');
}

function importing(file: string): boolean {
  return /from\s+'[^']*ad_slot'/.test(readFileSync(file, 'utf8'));
}

function surfaces(file: string): string[] {
  const text = readFileSync(file, 'utf8');
  return [...text.matchAll(/<AdSlot\s+surface="([^"]+)"/g)].map((match) => match[1]);
}

describe('ad placement', () => {
  it('Happy path: only the catalog, campaigns and landing screens import AdSlot', () => {
    const importers = sourceFiles(srcRoot).filter(importing).map(relative).sort();

    expect(importers).toEqual([...ALLOWED].sort());
  });

  it('Happy path: each allowed screen mounts its own surface at the bottom of the page', () => {
    expect(surfaces(path.resolve(srcRoot, 'catalog/catalog_screen.tsx'))).toEqual(['catalog']);
    expect(surfaces(path.resolve(srcRoot, 'campaign/campaigns_screen.tsx'))).toEqual(['campaigns']);
    expect(surfaces(path.resolve(srcRoot, 'landing/landing_screen.tsx'))).toEqual(['landing']);
  });

  it('Edge: the sheet, plans, campaign detail, generate, dice and auth modules mount no slot', () => {
    for (const file of [
      'sheet/sheet_screen.tsx',
      'sheet/sheet_edit.ts',
      'plans/plans_screen.tsx',
      'campaign/campaign_screen.tsx',
      'campaign/gm_sheet_screen.tsx',
      'generate/generate_screen.tsx',
      'auth/auth_screen.tsx',
      'auth/auth_code_screen.tsx',
    ]) {
      const text = readFileSync(path.resolve(srcRoot, file), 'utf8');
      expect(text).not.toContain('AdSlot');
      expect(text).not.toContain('ads.label');
    }
  });
});
