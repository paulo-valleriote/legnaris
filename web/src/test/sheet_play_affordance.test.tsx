import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { editName } from './dice_names';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { FieldsGroup } from '../sheet/preset/compose/fields_group';
import { IdentityHeader } from '../sheet/preset/compose/identity_header';
import { presetConfig, presetSheet, renderPreset } from './preset_fixtures';

const here = path.dirname(fileURLToPath(import.meta.url));
const presetCss = readFileSync(path.resolve(here, '../sheet/preset/preset.css'), 'utf8');

function ruleFor(selector: string): string {
  const start = presetCss.indexOf(selector);
  expect(start).toBeGreaterThanOrEqual(0);
  return presetCss.slice(start, presetCss.indexOf('}', start));
}

describe('the inline edit affordance', () => {
  it('Happy path: the control is a bordered box with a full control target', () => {
    const rule = ruleFor('.lg-inline-edit {');

    expect(rule).toMatch(
      /border:\s*var\(--lg-size-hairline\)\s+solid\s+var\(--lg-color-border-strong\)/,
    );
    expect(rule).toMatch(/border-radius:\s*var\(--lg-radius-sm\)/);
    expect(rule).toMatch(/background-color:\s*var\(--lg-color-raised\)/);
    expect(rule).toMatch(/min-width:\s*var\(--lg-size-control\)/);
    expect(rule).toMatch(/min-height:\s*var\(--lg-size-control\)/);
  });

  it('Edge: the focus ring is drawn on the box, not on a pencil icon', () => {
    const focus = presetCss.slice(presetCss.indexOf('.lg-inline-edit:focus-visible'));
    expect(focus.slice(0, focus.indexOf('}'))).toMatch(/outline:\s*var\(--lg-focus-width\)/);
    expect(presetCss).not.toContain('.lg-inline-edit__pencil');
  });

  it('Happy path: the header draws bordered controls and no pencil affordance', () => {
    renderPreset(
      <IdentityHeader header={{ name: 'identity.name', subtitle: 'identity.subtitle' }} />,
    );

    for (const label of ['Name', 'Subtitle']) {
      const control = screen.getByRole('button', { name: editName(label) });
      expect(control).toHaveClass('lg-inline-edit');
      expect(control.querySelector('.lg-inline-edit__pencil')).toBeNull();
      expect(control.querySelector('svg')).toBeNull();
    }
  });

  it('Edge: a derived field draws a boxless readout, not an inline edit', () => {
    renderPreset(
      <FieldsGroup
        piece={{
          kind: 'fields',
          key: 'readouts',
          title: 'preset.fields.character',
          fields: [{ path: 'vitals.derivedMax' }],
        }}
      />,
    );

    const row = document.querySelector('[data-field-path="vitals.derivedMax"]');
    expect(row).not.toBeNull();
    expect(row).toHaveClass('lg-compose-fields__row--derived');
    expect(row?.querySelector('.lg-field__readonly--derived')).not.toBeNull();
    expect(row?.querySelector('.lg-inline-edit, input, select')).toBeNull();
  });

  it('Happy path: a preset header still renders through the config adapter', () => {
    const preset = presetFromConfig(presetConfig());
    renderPreset(<preset.Header />, { sheet: presetSheet() });

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Nova');
    expect(screen.getByRole('button', { name: editName('Subtitle') })).toHaveClass(
      'lg-inline-edit',
    );
  });
});
