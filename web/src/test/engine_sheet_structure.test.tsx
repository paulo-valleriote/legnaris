import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { buildSections, rowKey } from '../engine/field_grouping';
import { SheetForm } from '../engine/sheet_form';

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return { type: 'number', label: '', isEditable: true, defaultValue: 0, ...overrides };
}

function leaf(overrides: Partial<FieldSchema> = {}): FieldNode {
  return { kind: 'leaf', schema: schema(overrides) };
}

function group(
  children: Record<string, FieldNode>,
  metadata: { label?: string; columns?: readonly string[] } = {},
): FieldNode {
  return { kind: 'group', children, ...metadata };
}

const MARKUP = '<img src=x onerror=alert(1)>';
const LONG_LABEL = 'An exceedingly long field label '.repeat(8).trim();

const FIELDS: Record<string, FieldNode> = {
  identity: group(
    {
      name: leaf({ type: 'text', label: 'Name' }),
      level: leaf({ label: 'Level' }),
      long: leaf({ label: LONG_LABEL }),
    },
    { label: 'Identity' },
  ),
  abilities: group(
    { str: leaf({ label: 'Strength' }), dex: leaf({ label: 'Dexterity' }) },
    { label: 'Abilities', columns: ['str', 'dex'] },
  ),
  flags: group({ inspired: leaf({ type: 'bool', label: 'Inspired' }) }, { label: 'Flags' }),
  derived: group(
    {
      total: leaf({ label: 'Total', isEditable: false, formula: 'a + b' }),
      title: leaf({ type: 'text', label: 'Title', isEditable: false }),
    },
    { label: 'Derived' },
  ),
  mixed: group(
    {
      stealth: leaf({ label: 'Stealth', exclusiveGroup: 'skill' }),
      athletics: leaf({ label: 'Athletics', exclusiveGroup: 'skill' }),
    },
    { label: 'Mixed' },
  ),
};

const DATA: Record<string, unknown> = {
  identity: { name: 'Aragorn', level: 3, long: 1 },
  abilities: { str: 16, dex: 14 },
  flags: { inspired: 1 },
  derived: { total: 4, title: 'Ranger' },
  mixed: { stealth: 1, athletics: 0 },
};

function rowsOf(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>('[data-row-key]'));
}

describe('sheet section and row wrappers (the stable units of a future Layout)', () => {
  it('Happy path: every section is one wrapper keyed by its top-level key, holding a title and a rows container', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const sections = Array.from(container.querySelectorAll<HTMLElement>('[data-section-key]'));
    expect(sections.map((section) => section.dataset.sectionKey)).toEqual([
      'identity',
      'abilities',
      'flags',
      'derived',
      'mixed',
    ]);
    for (const section of sections) {
      expect(section.tagName).toBe('SECTION');
      expect(section.querySelectorAll(':scope > h2.lg-section__title')).toHaveLength(1);
      expect(section.querySelectorAll(':scope > .lg-section__rows')).toHaveLength(1);
    }
  });

  it('Happy path: every row is one wrapper inside its section carrying a key and a kind', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const rows = rowsOf(container);
    expect(rows.map((row) => [row.dataset.rowKey, row.dataset.rowKind])).toEqual([
      ['identity.name', 'normal'],
      ['identity.level', 'normal'],
      ['identity.long', 'normal'],
      ['abilities', 'table'],
      ['flags.inspired', 'normal'],
      ['derived.total', 'normal'],
      ['derived.title', 'normal'],
      ['mixed#skill', 'exclusive'],
    ]);
    for (const row of rows) {
      expect(row.closest('.lg-section__rows')).not.toBeNull();
      expect(row).toHaveClass('lg-row');
    }
  });

  it('Edge: row keys are unique across the sheet', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const keys = rowsOf(container).map((row) => row.dataset.rowKey);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it('Edge: keys and wrapper identity survive a value change and a control-source switch', () => {
    const { container, rerender } = render(
      <SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />,
    );
    const before = rowsOf(container);
    const keysBefore = before.map((row) => row.dataset.rowKey);

    rerender(
      <SheetForm
        fields={FIELDS}
        data={{ ...DATA, identity: { name: 'Boromir', level: 9, long: 2 } }}
        onChanged={vi.fn()}
      />,
    );

    const after = rowsOf(container);
    expect(after.map((row) => row.dataset.rowKey)).toEqual(keysBefore);
    after.forEach((row, index) => expect(row).toBe(before[index]));

    const { container: builderContainer } = render(
      <SheetForm
        fields={FIELDS}
        controlBuilder={() => null}
        exclusiveControlBuilder={() => null}
      />,
    );
    expect(rowsOf(builderContainer).map((row) => row.dataset.rowKey)).toEqual(keysBefore);
  });

  it('Edge: the same exclusive id in two sections yields two distinct row keys', () => {
    const fields: Record<string, FieldNode> = {
      one: group({
        a: leaf({ label: 'A', exclusiveGroup: 'pick' }),
        b: leaf({ label: 'B', exclusiveGroup: 'pick' }),
      }),
      two: group({
        a: leaf({ label: 'A', exclusiveGroup: 'pick' }),
        b: leaf({ label: 'B', exclusiveGroup: 'pick' }),
      }),
    };
    const { container } = render(<SheetForm fields={fields} data={{}} onChanged={vi.fn()} />);

    expect(rowsOf(container).map((row) => row.dataset.rowKey)).toEqual(['one#pick', 'two#pick']);
  });

  it('Edge: a group with no leaf renders no section wrapper, and an empty tree renders none at all', () => {
    const { container } = render(
      <SheetForm
        fields={{ empty: group({}), outer: group({ inner: group({}) }), ...FIELDS }}
        data={DATA}
        onChanged={vi.fn()}
      />,
    );

    expect(container.querySelector('[data-section-key="empty"]')).toBeNull();
    expect(container.querySelector('[data-section-key="outer"]')).toBeNull();
    expect(container.querySelectorAll('[data-section-key]')).toHaveLength(5);

    const { container: none } = render(<SheetForm fields={{}} data={{}} onChanged={vi.fn()} />);
    expect(none.querySelectorAll('[data-section-key]')).toHaveLength(0);
    expect(none.querySelectorAll('[data-row-key]')).toHaveLength(0);
  });

  it('Edge: a collapsed section keeps its wrapper key and its rows mounted inside a hidden container', () => {
    const { container } = render(
      <SheetForm
        fields={FIELDS}
        data={DATA}
        onChanged={vi.fn()}
        collapsedSectionKeys={['identity']}
        onToggleSection={vi.fn()}
      />,
    );

    const collapsed = container.querySelector('[data-section-key="identity"]');
    expect(collapsed).toHaveClass('lg-section--collapsed');
    expect(collapsed?.querySelectorAll('[data-row-key]')).toHaveLength(3);
    expect(collapsed?.querySelector('.lg-section__rows')).toHaveAttribute('hidden');
    expect(container.querySelectorAll('.lg-section__rows[hidden]')).toHaveLength(1);
  });

  it('Happy path: a table row owns its column header inside its own row wrapper', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const header = container.querySelector('.lg-field-table__header');
    expect(header).not.toBeNull();
    expect(header?.hasAttribute('data-row-key')).toBe(false);
    expect(header?.closest('[data-row-key]')?.getAttribute('data-row-key')).toBe('abilities');
  });

  it('Edge: the row mark seam lands inside the row wrapper it belongs to', () => {
    const rowMarkBuilder = vi.fn((paths: readonly string[]) =>
      paths.includes('identity.level') ? <span data-testid="mark">Modified</span> : null,
    );
    const { container } = render(
      <SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} rowMarkBuilder={rowMarkBuilder} />,
    );

    const row = container.querySelector('[data-row-key="identity.level"]');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByTestId('mark')).toBeInTheDocument();
    expect(screen.getAllByTestId('mark')).toHaveLength(1);
  });
});

describe('editable fields and derived fields look different', () => {
  it('Happy path: a derived field is a readout, an editable field is an input', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const total = screen.getByRole('group', { name: /^Total(:| |$)/ });
    expect(total).toHaveClass('lg-field__readonly--derived', 'lg-field__readonly--numeric');
    expect(total).not.toHaveClass('lg-field');
    expect(screen.queryByRole('textbox', { name: 'Total' })).not.toBeInTheDocument();

    const level = screen.getByRole('textbox', { name: 'Level' });
    expect(level).toHaveClass('lg-field');
    expect(level.closest('.lg-row')).not.toHaveClass('lg-row--derived');
    expect(container.querySelectorAll('.lg-field__readonly--derived')).toHaveLength(2);
  });

  it('Happy path: a row of derived fields is flagged so it can read as computed', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const derivedRows = container.querySelectorAll('.lg-row--derived');
    expect(Array.from(derivedRows).map((row) => (row as HTMLElement).dataset.rowKey)).toEqual([
      'derived.total',
      'derived.title',
    ]);
  });

  it('Edge: a table whose columns are all derived is a derived row, a mixed table is not', () => {
    const allDerived: Record<string, FieldNode> = {
      stats: group(
        {
          a: leaf({ label: 'A', isEditable: false }),
          b: leaf({ label: 'B', isEditable: false }),
        },
        { columns: ['a', 'b'] },
      ),
    };
    const mixed: Record<string, FieldNode> = {
      stats: group(
        { a: leaf({ label: 'A' }), b: leaf({ label: 'B', isEditable: false }) },
        { columns: ['a', 'b'] },
      ),
    };

    const first = render(<SheetForm fields={allDerived} data={{}} onChanged={vi.fn()} />);
    expect(first.container.querySelector('.lg-field-table__row')).toHaveClass('lg-row--derived');
    first.unmount();

    const second = render(<SheetForm fields={mixed} data={{}} onChanged={vi.fn()} />);
    expect(second.container.querySelector('.lg-field-table__row')).not.toHaveClass(
      'lg-row--derived',
    );
    expect(second.container.querySelectorAll('.lg-field__readonly--derived')).toHaveLength(1);
  });

  it('Edge: an editable leaf of an unknown type renders a read-only value that is not styled as derived', () => {
    const fields: Record<string, FieldNode> = {
      misc: group({ mystery: leaf({ type: 'fancy', label: 'Mystery' }) }),
    };
    render(<SheetForm fields={fields} data={{ misc: { mystery: 7 } }} onChanged={vi.fn()} />);

    expect(screen.getByRole('group', { name: /^Mystery(:| |$)/ })).not.toHaveClass(
      'lg-field__readonly--derived',
    );
  });

  it('Happy path: a boolean stays one native checkbox that reports its toggle', () => {
    const onChanged = vi.fn();
    render(<SheetForm fields={FIELDS} data={DATA} onChanged={onChanged} />);

    const toggle = screen.getByRole('checkbox', { name: 'Inspired' });
    expect(toggle).toHaveClass('lg-field__toggle');
    expect(toggle).toBeChecked();
    fireEvent.click(toggle);
    expect(onChanged).toHaveBeenCalledWith('flags.inspired', false);
  });
});

describe('labels render as literal text', () => {
  it('Edge: a very long label is rendered whole in a wrapping label element', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const row = container.querySelector('[data-row-key="identity.long"]');
    expect(row?.querySelector('.lg-field-row__label-text')).toHaveTextContent(LONG_LABEL);
    expect(screen.getByRole('textbox', { name: LONG_LABEL })).toBeInTheDocument();
  });

  it('Edge: markup in a section, row or column label stays text and creates no element or injected key', () => {
    const fields: Record<string, FieldNode> = {
      '"><b id=x>': group(
        {
          [MARKUP]: leaf({ label: MARKUP }),
          cols: group({ c: leaf({ label: MARKUP }) }, { label: MARKUP, columns: ['c'] }),
        },
        { label: MARKUP },
      ),
    };
    const { container } = render(<SheetForm fields={fields} data={{}} onChanged={vi.fn()} />);

    expect(screen.getByRole('heading', { name: MARKUP })).toBeInTheDocument();
    expect(screen.getAllByText(MARKUP).length).toBeGreaterThan(0);
    expect(container.querySelectorAll('img, b, script')).toHaveLength(0);
    expect(container.querySelector('#x')).toBeNull();
    expect(container.querySelector('[data-section-key]')?.getAttribute('data-section-key')).toBe(
      '"><b id=x>',
    );
  });
});

describe('rowKey', () => {
  it('Happy path: derives a stable key for each row kind', () => {
    const sections = buildSections(FIELDS);
    const keys = sections.flatMap((section) => section.rows.map((row) => rowKey(row)));

    expect(keys).toContain('identity.level');
    expect(keys).toContain('abilities');
    expect(keys).toContain('mixed#skill');
  });

  it('Edge: a nested table is keyed by the path of the group that declares its columns', () => {
    const fields: Record<string, FieldNode> = {
      outer: group({
        inner: group({ a: leaf({ label: 'A' }) }, { columns: ['a'] }),
      }),
    };
    const [section] = buildSections(fields);

    expect(rowKey(section.rows[0])).toBe('outer.inner');
  });

  it('Edge: a single-member exclusive id is not collapsed and keys as a lone field', () => {
    const fields: Record<string, FieldNode> = {
      solo: group({ a: leaf({ label: 'A', exclusiveGroup: 'only' }) }),
    };
    const [section] = buildSections(fields);

    expect(rowKey(section.rows[0])).toBe('solo.a');
  });
});
