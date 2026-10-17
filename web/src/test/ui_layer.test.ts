import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const uiDir = path.resolve(here, '../ui');

function uiSourceFiles(): string[] {
  return readdirSync(uiDir)
    .filter(
      (n) => n.endsWith('.tsx') || n.endsWith('.ts') || n.endsWith('.css')
    )
    .map((n) => path.join(uiDir, n));
}

describe('shared layer governance (UIP-01, UIP-02, UIP-03)', () => {
  it('Edge: a module that imports a feature directory is reported', () => {
    const files = uiSourceFiles();
    for (const f of files) {
      const text = readFileSync(f, 'utf8');
      // The layer must not reference feature directories.
      const importLines = text
        .split('\n')
        .filter((line) => line.includes('import'));
      for (const line of importLines) {
        expect(line).not.toMatch(/\.\.(\/shell|\/catalog|\/engine|\/auth|\/sheet|\/library)\/);
      }
    }
  });

  it('Edge: a barrel under src/ui/ is reported', () => {
    const files = readdirSync(uiDir);
    expect(files).not.toContain('index.ts');
    expect(files).not.toContain('index.tsx');
  });

  it('Happy path: the layer holds the five primitive modules plus focus trap', () => {
    const files = readdirSync(uiDir);
    ['button.tsx', 'field.tsx', 'checkbox.tsx', 'dialog.tsx', 'toast.tsx', 'focus_trap.ts'].forEach((f) => {
      expect(files).toContain(f);
    });
  });

  it('Happy path: the layer holds the five primitive modules', () => {
    const files = readdirSync(uiDir);
    expect(files).toContain('button.tsx');
  });
});
