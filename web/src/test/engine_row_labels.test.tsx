import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { parseTemplateDetail } from '../api/models/template';
import type { SheetField } from '../engine/field_grouping';
import { SheetFieldControl } from '../engine/field_control';
import { SheetForm } from '../engine/sheet_form';

function tree(children: Record<string, unknown>): ReturnType<typeof parseTemplateDetail>['fields'] {
  return parseTemplateDetail({
    id: 'fantasy_v1',
    name: 'Fantasy',
    default: true,
    fields: { stats: { label: 'Stats', children } },
  }).fields;
}

const derivedLeaf = (label: string, extra: Record<string, unknown> = {}) => ({
  schema: { type: 'number', label, is_editable: false, formula: 'a + 1', default: 0, ...extra },
});

function tableGroup(): Record<string, unknown> {
  return {
    label: 'Athletics',
    columns: ['base', 'total'],
    children: {
      base: { schema: { type: 'number', label: 'Base', is_editable: true, default: 0 } },
      total: derivedLeaf('Total', { column_label: 'Total' }),
    },
  };
}

describe('derived readout names', () => {
  it('Happy path: a lone derived readout is named by the visible row label and carries no aria-label', () => {
    render(<SheetForm fields={tree({ total: derivedLeaf('Total') })} data={{}} />);

    const readout = screen.getByRole('group', { name: /^Total(:| |$)/ });
    const labelId = readout.getAttribute('aria-labelledby')?.split(' ')[0];
    expect(labelId).toBeTruthy();
    const target = document.getElementById(labelId ?? '');
    expect(target).not.toBeNull();
    expect(target).toHaveClass('lg-field-row__label-text');
    expect(target).toHaveTextContent('Total');
    expect(readout).not.toHaveAttribute('aria-label');
    expect(readout).toHaveAccessibleName('Total');
  });

  it('Edge: the name is the label and the value, never the formula, the caption or a preview mark', () => {
    render(
      <SheetForm
        fields={tree({ total: derivedLeaf('Total') })}
        data={{ stats: { total: 9 } }}
        rowMarkBuilder={() => (
          <span className="lg-field-row__mark" data-mark="modified">
            Modified
          </span>
        )}
      />,
    );

    const readout = screen.getByRole('group');
    expect(readout).toHaveAccessibleName('Total 9');
    expect(readout).not.toHaveAccessibleName(expect.stringContaining('a + 1'));
    expect(screen.getAllByText('Total')).toHaveLength(1);
    expect(readout.closest('[data-row-key]')?.querySelector('.lg-field-row__mark')).not.toBeNull();
  });

  it('Edge: every label id is unique across the form and resolves to its own row', () => {
    const { container } = render(
      <SheetForm
        fields={tree({
          one: derivedLeaf('One'),
          two: derivedLeaf('Two'),
          three: { schema: { type: 'text', label: 'Three', is_editable: true, default: 0 } },
        })}
        data={{}}
      />,
    );

    const ids = Array.from(container.querySelectorAll('.lg-field-row__label-text')).map(
      (element) => element.id,
    );
    expect(ids).toHaveLength(3);
    expect(new Set(ids).size).toBe(3);
    for (const readout of container.querySelectorAll('[role="group"][aria-labelledby]')) {
      const row = readout.closest('[data-row-key]');
      const labelId = readout.getAttribute('aria-labelledby')?.split(' ')[0] ?? '';
      expect(row?.querySelector(`[id="${labelId}"]`)).not.toBeNull();
    }
  });

  it('Edge: two forms on one page never share a label id', () => {
    const fields = tree({ total: derivedLeaf('Total') });
    render(
      <>
        <SheetForm fields={fields} data={{}} />
        <SheetForm fields={fields} data={{}} />
      </>,
    );

    const readouts = screen.getAllByRole('group', { name: /^Total(:| |$)/ });
    expect(readouts).toHaveLength(2);
    expect(readouts[0]?.getAttribute('aria-labelledby')).not.toBe(
      readouts[1]?.getAttribute('aria-labelledby'),
    );
    for (const readout of readouts) {
      const labelId = readout.getAttribute('aria-labelledby')?.split(' ')[0] ?? '';
      expect(readout.closest('[data-row-key]')?.contains(document.getElementById(labelId))).toBe(
        true,
      );
    }
  });

  it('Edge: a key with whitespace still yields an id without whitespace that resolves', () => {
    render(<SheetForm fields={tree({ 'bonus total': derivedLeaf('Bonus total') })} data={{}} />);

    const readout = screen.getByRole('group', { name: /^Bonus total(:| |$)/ });
    const [id = '', valueId = ''] = (readout.getAttribute('aria-labelledby') ?? '').split(' ');
    expect(id).not.toMatch(/\s/);
    expect(valueId).not.toBe('');
    expect(document.getElementById(id)).toHaveTextContent('Bonus total');
  });

  it('Edge: a table cell has no row label of its own and keeps its aria-label', () => {
    const { container } = render(
      <SheetForm fields={tree({ athletics: tableGroup() })} data={{}} />,
    );

    const cell = container.querySelector<HTMLElement>(
      '[data-row-kind="table"] .lg-field__readonly--derived',
    );
    expect(cell).not.toBeNull();
    expect(cell).toHaveAttribute('aria-label', 'Total');
    expect(cell).not.toHaveAttribute('aria-labelledby');
    expect(screen.getByRole('group', { name: /^Total(:| |$)/ })).toBe(cell);
  });

  it('Edge: builder mode hands the label id to a lone row and none to a table cell', () => {
    const seen: Array<readonly [string, string | undefined]> = [];
    const controlBuilder = vi.fn((field: SheetField, labelId?: string) => {
      seen.push([field.path, labelId]);
      return null;
    });
    render(
      <SheetForm
        fields={tree({ strength: derivedLeaf('Strength'), athletics: tableGroup() })}
        controlBuilder={controlBuilder}
        exclusiveControlBuilder={() => null}
      />,
    );

    const lone = seen.find(([path]) => path === 'stats.strength');
    expect(lone?.[1]).toBeTruthy();
    expect(document.getElementById(lone?.[1] ?? '')).toHaveTextContent('Strength');
    const cells = seen.filter(([path]) => path.startsWith('stats.athletics'));
    expect(cells).toHaveLength(2);
    for (const [, labelId] of cells) {
      expect(labelId).toBeUndefined();
    }
  });

  it('Edge: a control given a label id that resolves nowhere is still rendered with that reference', () => {
    render(
      <SheetFieldControl
        path="stats.total"
        schema={{
          type: 'number',
          label: 'Total',
          isEditable: false,
          defaultValue: 0,
          formula: 'a',
        }}
        value={1}
        labelId="missing-label"
      />,
    );

    const readout = screen.getByRole('group');
    expect(readout.getAttribute('aria-labelledby')?.split(' ')[0]).toBe('missing-label');
    expect(readout).not.toHaveAttribute('aria-label');
  });
});
