import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { FieldNode } from '../api/models/template';
import { needsTableHeader, buildSections } from '../engine/field_grouping';
import type { TableRow } from '../engine/field_grouping';
import { resolveArrangement } from '../engine/layout';
import type { RowFrameProps, SectionFrameProps } from '../engine/sheet_form';
import { SheetForm } from '../engine/sheet_form';
import { ARRANGE_FIELDS, arrangeSections } from './layout_fixtures';

const sections = arrangeSections();

function arranged(layout: Parameters<typeof resolveArrangement>[1]) {
  return resolveArrangement(sections, layout);
}

function draw(arrangement?: ReturnType<typeof arranged>, extra: Record<string, unknown> = {}) {
  return render(
    <SheetForm
      fields={ARRANGE_FIELDS}
      data={{}}
      onChanged={vi.fn()}
      arrangement={arrangement}
      {...extra}
    />,
  );
}

const sectionKeys = (container: HTMLElement) =>
  Array.from(container.querySelectorAll<HTMLElement>('[data-section-key]')).map(
    (section) => section.dataset.sectionKey,
  );

const rowKeys = (scope: ParentNode) =>
  Array.from(scope.querySelectorAll<HTMLElement>('[data-row-key]')).map(
    (row) => row.dataset.rowKey,
  );

const sectionRows = (container: HTMLElement, key: string) =>
  rowKeys(container.querySelector<HTMLElement>(`[data-section-key="${key}"]`) as HTMLElement);

describe('SheetForm follows an arrangement', () => {
  it('Edge: no arrangement draws the Template order with no page-level zone', () => {
    const { container } = draw();

    expect(sectionKeys(container)).toEqual(['alpha', 'beta', 'gamma']);
    expect(container.querySelector('[data-zone]')).toBeNull();
    expect(rowKeys(container)).toEqual([
      'alpha.one',
      'alpha.two',
      'alpha.three',
      'beta.note',
      'beta.stats',
      'beta#pick',
      'gamma.solo',
    ]);
  });

  it('Happy path: sections are drawn in the arranged order', () => {
    const { container } = draw(arranged({ sectionOrder: ['gamma', 'alpha'], pinned: [] }));

    expect(sectionKeys(container)).toEqual(['gamma', 'alpha', 'beta']);
  });

  it('Happy path: a pinned Row rises to the top of its own Section, once', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['alpha.three'] }));

    expect(sectionRows(container, 'alpha')).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(container.querySelectorAll('[data-row-key="alpha.three"]')).toHaveLength(1);
    expect(sectionKeys(container)).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Edge: pinned Rows are ordered by pin time and the rest keep Template order', () => {
    const { container } = draw(
      arranged({ sectionOrder: [], pinned: ['alpha.three', 'alpha.two'] }),
    );

    expect(sectionRows(container, 'alpha')).toEqual(['alpha.three', 'alpha.two', 'alpha.one']);
  });

  it('Edge: pins in several Sections never leave their Section', () => {
    const { container } = draw(
      arranged({ sectionOrder: [], pinned: ['gamma.solo', 'beta#pick', 'alpha.two'] }),
    );

    expect(sectionRows(container, 'alpha')).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
    expect(sectionRows(container, 'beta')).toEqual(['beta#pick', 'beta.note', 'beta.stats']);
    expect(sectionRows(container, 'gamma')).toEqual(['gamma.solo']);
  });

  it('Edge: a pinned key naming no Row, and a duplicated pin, are ignored', () => {
    const { container } = draw(
      arranged({ sectionOrder: [], pinned: ['ghost', 'alpha.two', 'alpha.two'] }),
    );

    expect(sectionRows(container, 'alpha')).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
    expect(container.querySelectorAll('[data-row-key="alpha.two"]')).toHaveLength(1);
  });

  it('Edge: a Section whose every Row is pinned still renders normally', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['gamma.solo'] }));

    expect(sectionKeys(container)).toEqual(['alpha', 'beta', 'gamma']);
    expect(sectionRows(container, 'gamma')).toEqual(['gamma.solo']);
    expect(screen.getByRole('heading', { name: 'Gamma' })).toBeInTheDocument();
  });

  it('Edge: pinning every Row of the sheet keeps every Section and Row', () => {
    const pinned = sections.flatMap((section) =>
      section.rows.map((row) =>
        row.kind === 'normal' ? row.field.path : row.kind === 'table' ? 'beta.stats' : 'beta#pick',
      ),
    );
    const { container } = draw(arranged({ sectionOrder: [], pinned }));

    expect(sectionKeys(container)).toEqual(['alpha', 'beta', 'gamma']);
    expect(rowKeys(container)).toHaveLength(7);
  });

  it('Edge: no Row shows a caption or a star outside Arrange mode', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['alpha.two'] }));

    expect(container.querySelector('.lg-row__caption')).toBeNull();
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('Edge: a value change keeps a pinned Row wrapper and its controls', () => {
    const arrangement = arranged({ sectionOrder: [], pinned: ['alpha.two'] });
    const { container, rerender } = draw(arrangement);
    const before = container.querySelector('[data-row-key="alpha.two"]');

    rerender(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{ alpha: { two: 9 } }}
        onChanged={vi.fn()}
        arrangement={arrangement}
      />,
    );

    expect(container.querySelector('[data-row-key="alpha.two"]')).toBe(before);
    expect(screen.getByRole('textbox', { name: 'Two' })).toHaveValue('9');
  });
});

describe('a pinned Row is marked beside its label', () => {
  const marks = (scope: ParentNode) => scope.querySelectorAll('.lg-pin-mark');
  const rowOf = (container: HTMLElement, key: string) =>
    container.querySelector<HTMLElement>(`[data-row-key="${key}"]`) as HTMLElement;

  it('Happy path: a pinned Row carries one star with the hidden name Pinned, after its label', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['alpha.two'] }));
    const label = rowOf(container, 'alpha.two').querySelector(
      '.lg-field-row__label',
    ) as HTMLElement;

    expect(marks(label)).toHaveLength(1);
    expect(label.firstElementChild).toHaveClass('lg-field-row__label-text');
    expect(label.children[1]).toHaveClass('lg-pin-mark');
    expect(label.children[1]).toHaveTextContent('Pinned');
    expect(label.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('Edge: nothing is marked without an arrangement or with nothing pinned', () => {
    expect(marks(draw().container)).toHaveLength(0);
    expect(marks(draw(arranged({ sectionOrder: [], pinned: [] })).container)).toHaveLength(0);
  });

  it('Edge: only the pinned Rows are marked, and a pin naming no Row marks none', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['alpha.two', 'ghost'] }));

    expect(marks(container)).toHaveLength(1);
    expect(marks(rowOf(container, 'alpha.two'))).toHaveLength(1);
  });

  it('Edge: exclusive and table Rows are marked too, once each', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['beta#pick', 'beta.stats'] }));

    expect(marks(rowOf(container, 'beta#pick'))).toHaveLength(1);
    expect(marks(rowOf(container, 'beta.stats'))).toHaveLength(1);
    expect(marks(container)).toHaveLength(2);
  });

  it('Edge: the mark sits before a preview mark and keeps it', () => {
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['alpha.two'] }), {
      rowMarkBuilder: () => <span data-testid="preview-mark">Modified</span>,
    });
    const label = rowOf(container, 'alpha.two').querySelector(
      '.lg-field-row__label',
    ) as HTMLElement;

    expect(Array.from(label.children).map((child) => child.className)).toEqual([
      'lg-field-row__label-text',
      'lg-pin-mark',
      '',
    ]);
  });

  it('Edge: a Row that is unpinned loses its mark on the next draw', () => {
    const { container, rerender } = draw(arranged({ sectionOrder: [], pinned: ['alpha.two'] }));
    rerender(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged({ sectionOrder: [], pinned: [] })}
      />,
    );

    expect(marks(container)).toHaveLength(0);
  });

  it('Edge: a replaced Row frame decides the chrome itself, so no mark is drawn by the engine', () => {
    const Row = ({ rowKey, pinned, label }: RowFrameProps) => (
      <div data-row-key={rowKey}>{`${label}${pinned ? ' *' : ''}`}</div>
    );
    const { container } = draw(arranged({ sectionOrder: [], pinned: ['alpha.two'] }), {
      frames: { Row },
    });

    expect(marks(container)).toHaveLength(0);
  });
});

describe('table Rows own their column header', () => {
  const headerCount = (scope: ParentNode) =>
    scope.querySelectorAll('.lg-field-table__header').length;

  const tableFields: Record<string, FieldNode> = {
    t: {
      kind: 'group',
      label: 'T',
      children: {
        a: { kind: 'group', label: 'A', columns: ['x'], children: { x: leaf('X') } },
        b: { kind: 'group', label: 'B', columns: ['x'], children: { x: leaf('X') } },
        c: { kind: 'group', label: 'C', columns: ['y'], children: { y: leaf('Y') } },
        note: leaf('Note'),
      },
    },
  };
  const tableSections = buildSections(tableFields);
  const TABLE_KEYS = ['t.a', 't.b', 't.c', 't.note'];

  function leaf(label: string): FieldNode {
    return { kind: 'leaf', schema: { type: 'number', label, isEditable: true, defaultValue: 0 } };
  }

  function drawTables(pinned: readonly string[]) {
    return render(
      <SheetForm
        fields={tableFields}
        data={{}}
        onChanged={vi.fn()}
        arrangement={resolveArrangement(tableSections, { sectionOrder: [], pinned })}
      />,
    );
  }

  const columns: Record<string, string> = { 't.a': 'x', 't.b': 'x', 't.c': 'y' };

  function headersInDrawOrder(container: HTMLElement): [string, number][] {
    return Array.from(container.querySelectorAll<HTMLElement>('[data-row-key]')).map((row) => [
      row.dataset.rowKey as string,
      headerCount(row),
    ]);
  }

  function expectOneHeaderPerRun(container: HTMLElement) {
    const drawn = headersInDrawOrder(container);
    drawn.forEach(([key, headers], index) => {
      const column = columns[key];
      if (column === undefined) {
        expect(headers).toBe(0);
        return;
      }
      const previous = drawn[index - 1]?.[0];
      const startsRun = previous === undefined || columns[previous] !== column;
      expect([key, headers]).toEqual([key, startsRun ? 1 : 0]);
    });
  }

  function* pinSequences(keys: readonly string[]): Generator<string[]> {
    yield [];
    for (const key of keys) {
      for (const rest of pinSequences(keys.filter((other) => other !== key))) {
        yield [key, ...rest];
      }
    }
  }

  it('Happy path: a run of table Rows with the same columns shares the header of its first Row', () => {
    const { container } = drawTables([]);

    expect(headersInDrawOrder(container)).toEqual([
      ['t.a', 1],
      ['t.b', 0],
      ['t.c', 1],
      ['t.note', 0],
    ]);
  });

  it('Edge: a Row pinned to the top starts its own run and the run it left keeps one header', () => {
    const { container } = drawTables(['t.b']);

    expect(headersInDrawOrder(container)).toEqual([
      ['t.b', 1],
      ['t.a', 0],
      ['t.c', 1],
      ['t.note', 0],
    ]);
  });

  it('Edge: a Row pinned next to a different-column table gets its own header, and so does its new neighbour', () => {
    const { container } = drawTables(['t.c']);

    expect(headersInDrawOrder(container)).toEqual([
      ['t.c', 1],
      ['t.a', 1],
      ['t.b', 0],
      ['t.note', 0],
    ]);
  });

  it('Edge: a non-table Row pinned to the top leaves the table runs untouched', () => {
    const { container } = drawTables(['t.note']);

    expect(headersInDrawOrder(container)).toEqual([
      ['t.note', 0],
      ['t.a', 1],
      ['t.b', 0],
      ['t.c', 1],
    ]);
  });

  it('Edge: a pin that separates two same-column Rows gives each its own header', () => {
    const { container } = drawTables(['t.a', 't.c']);

    expect(headersInDrawOrder(container)).toEqual([
      ['t.a', 1],
      ['t.c', 1],
      ['t.b', 1],
      ['t.note', 0],
    ]);
  });

  it('Edge: every pin sequence draws exactly one header per run and none elsewhere', () => {
    let checked = 0;
    for (const pinned of pinSequences(TABLE_KEYS)) {
      const { container, unmount } = drawTables(pinned);
      expectOneHeaderPerRun(container);
      unmount();
      checked += 1;
    }
    expect(checked).toBeGreaterThan(60);
  });

  it('Edge: the header is inside the row wrapper and carries no row key of its own', () => {
    const { container } = drawTables(['t.b']);

    for (const header of container.querySelectorAll('.lg-field-table__header')) {
      expect(header.hasAttribute('data-row-key')).toBe(false);
      expect(header.closest('[data-row-key]')).not.toBeNull();
    }
  });

  describe('needsTableHeader', () => {
    const table = (label: string, column: string): TableRow => ({
      kind: 'table',
      label,
      fields: [
        {
          path: `t.${label}.${column}`,
          schema: { type: 'number', label: column, isEditable: true, defaultValue: 0 },
        },
      ],
    });

    it.each([
      ['no previous Row', undefined, true],
      [
        'a previous Row of another kind',
        {
          kind: 'normal' as const,
          field: {
            path: 'n',
            schema: { type: 'text', label: 'n', isEditable: true, defaultValue: 0 },
          },
        },
        true,
      ],
      ['a previous table with the same columns', table('p', 'x'), false],
      ['a previous table with other columns', table('p', 'z'), true],
    ])('Edge: %s', (_name, previous, expected) => {
      expect(needsTableHeader(previous, table('q', 'x'))).toBe(expected);
    });
  });
});

describe('SheetForm frames seam', () => {
  it('Happy path: Section and Row frames receive the structure and replace the wrappers', () => {
    const seenRows: RowFrameProps[] = [];
    const seenSections: SectionFrameProps[] = [];
    const frames = {
      Root: ({
        children,
        sectionKeys,
      }: {
        children: React.ReactNode;
        sectionKeys: readonly string[];
      }) => (
        <div data-testid="root" data-keys={sectionKeys.join(',')}>
          {children}
        </div>
      ),
      Section: (props: SectionFrameProps) => {
        seenSections.push(props);
        return (
          <article data-framed-section={props.sectionKey}>
            {props.title}
            {props.children}
          </article>
        );
      },
      Row: (props: RowFrameProps) => {
        seenRows.push(props);
        return <li data-framed-row={props.rowKey}>{props.children}</li>;
      },
    };
    const { container } = draw(arranged({ sectionOrder: ['gamma'], pinned: ['alpha.one'] }), {
      frames,
    });

    expect(screen.getByTestId('root')).toHaveAttribute('data-keys', 'gamma,alpha,beta');
    expect(container.querySelectorAll('[data-framed-section]')).toHaveLength(3);
    expect(seenSections.map((s) => [s.sectionKey, s.label])).toEqual([
      ['gamma', 'Gamma'],
      ['alpha', 'Alpha'],
      ['beta', 'Beta'],
    ]);
    expect(seenRows.find((row) => row.rowKey === 'alpha.one')).toMatchObject({
      pinned: true,
      sectionKey: 'alpha',
      label: 'One',
    });
    expect(seenRows.find((row) => row.rowKey === 'alpha.two')).toMatchObject({ pinned: false });
    expect(seenRows.find((row) => row.rowKey === 'beta#pick')).toMatchObject({
      kind: 'exclusive',
      label: 'Pick',
    });
    expect(seenRows.find((row) => row.rowKey === 'beta.stats')).toMatchObject({
      kind: 'table',
      label: 'Stats',
    });
  });

  it('Edge: a Row frame is handed the Rows of a Section in pinned-first order', () => {
    const order: string[] = [];
    const frames = {
      Row: (props: RowFrameProps) => {
        order.push(props.rowKey);
        return <div data-row-key={props.rowKey}>{props.children}</div>;
      },
    };
    draw(arranged({ sectionOrder: [], pinned: ['alpha.three'] }), { frames });

    expect(order.slice(0, 3)).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
  });

  it('Edge: markup in a Section or Row label reaches the frames as literal text', () => {
    const markup = '<img src=x onerror=alert(1)>';
    const fields: Record<string, FieldNode> = {
      s: {
        kind: 'group',
        label: markup,
        children: {
          f: {
            kind: 'leaf',
            schema: { type: 'number', label: markup, isEditable: true, defaultValue: 0 },
          },
        },
      },
    };
    const seen: RowFrameProps[] = [];
    const frames = {
      Row: (props: RowFrameProps) => {
        seen.push(props);
        return <div data-row-key={props.rowKey}>{props.label}</div>;
      },
    };
    const { container } = render(
      <SheetForm fields={fields} data={{}} onChanged={vi.fn()} frames={frames} />,
    );

    expect(seen[0].label).toBe(markup);
    expect(screen.getAllByText(markup).length).toBeGreaterThan(0);
    expect(container.querySelectorAll('img')).toHaveLength(0);
  });
});
