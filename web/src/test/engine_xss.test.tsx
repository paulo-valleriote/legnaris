import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { FieldSchema } from '../api/models/template';
import { SheetFieldControl } from '../engine/field_control';
import { SheetExclusiveControl } from '../engine/exclusive_control';

const engineDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../engine');

function engineSources(): string[] {
  return readdirSync(engineDir)
    .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
    .map((name) => path.join(engineDir, name));
}

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return { type: 'number', label: '', isEditable: true, defaultValue: 0, ...overrides };
}

const MARKUP = '<script>alert(1)</script>';

describe('markup in data renders as text (ENG-29, SC-005)', () => {
  it('Edge: a field value of markup renders as literal text and creates no script element', () => {
    const { container } = render(
      <SheetFieldControl
        path="identity.name"
        schema={schema({ type: 'text', label: 'Name', isEditable: false })}
        value={MARKUP}
      />,
    );

    expect(screen.getByRole('group', { name: 'Name' })).toHaveTextContent(MARKUP);
    expect(container.querySelectorAll('script')).toHaveLength(0);
  });

  it('Edge: an option label of markup renders as literal text and creates no script element', () => {
    const { container } = render(
      <SheetExclusiveControl
        group={{
          id: 'ability',
          options: [
            { path: 'stats.stealth', schema: schema({ label: 'Stealth', optionLabel: MARKUP }) },
          ],
        }}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByRole('option', { name: MARKUP })).toBeInTheDocument();
    expect(container.querySelectorAll('script')).toHaveLength(0);
  });

  it('Edge: the engine sources contain no DOM-injection sink', () => {
    const sources = engineSources();
    expect(sources.length).toBeGreaterThan(0);

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');
      expect(text).not.toContain('dangerouslySetInnerHTML');
      expect(text).not.toContain('innerHTML');
      expect(text).not.toContain('document.write');
      expect(text).not.toContain('new Function');
      expect(text).not.toMatch(/\beval\s*\(/);
    }
  });
});
