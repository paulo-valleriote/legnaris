import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { buildSections } from '../engine/field_grouping';
import { resolveArrangement } from '../engine/layout';
import type { SectionFrameProps } from '../engine/sheet_form';
import { SheetForm } from '../engine/sheet_form';
import { ARRANGE_FIELDS } from './layout_fixtures';

function numberLeaf(label: string, overrides: Partial<FieldSchema> = {}): FieldNode {
  return {
    kind: 'leaf',
    schema: { type: 'number', label, isEditable: true, defaultValue: 0, ...overrides },
  };
}

const TABLES: Record<string, FieldNode> = {
  grid: {
    kind: 'group',
    label: 'Grid',
    children: {
      first: {
        kind: 'group',
        label: 'First',
        columns: ['str', 'dex'],
        children: { str: numberLeaf('Strength'), dex: numberLeaf('Dexterity') },
      },
      middle: numberLeaf('Middle'),
      last: {
        kind: 'group',
        label: 'Last',
        columns: ['str', 'dex'],
        children: { str: numberLeaf('Strength'), dex: numberLeaf('Dexterity') },
      },
    },
  },
  other: { kind: 'group', label: 'Other', children: { solo: numberLeaf('Solo') } },
};

function arranged(pinned: string[], fields = ARRANGE_FIELDS) {
  return resolveArrangement(buildSections(fields), { sectionOrder: [], pinned });
}

const sectionKeys = (container: HTMLElement) =>
  Array.from(container.querySelectorAll<HTMLElement>('[data-section-key]')).map(
    (section) => section.dataset.sectionKey,
  );

const rowKeys = (scope: ParentNode) =>
  Array.from(scope.querySelectorAll<HTMLElement>('[data-row-key]')).map(
    (row) => row.dataset.rowKey,
  );

function Harness({ initial = [] as string[] }: { readonly initial?: string[] }) {
  const [collapsed, setCollapsed] = useState<string[]>(initial);
  return (
    <SheetForm
      fields={ARRANGE_FIELDS}
      data={{}}
      onChanged={vi.fn()}
      collapsedSectionKeys={collapsed}
      onToggleSection={(key) =>
        setCollapsed((current) =>
          current.includes(key) ? current.filter((entry) => entry !== key) : [...current, key],
        )
      }
    />
  );
}

describe('a Section title as a disclosure control', () => {
  it('Edge: without a toggle handler the title is plain text, never a button', () => {
    render(<SheetForm fields={ARRANGE_FIELDS} data={{}} onChanged={vi.fn()} />);

    expect(screen.getByRole('heading', { name: 'Alpha' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Alpha' })).not.toBeInTheDocument();
    expect(document.querySelector('[aria-expanded]')).toBeNull();
  });

  it('Happy path: every title is a button inside its heading, expanded by default', () => {
    render(<Harness />);

    for (const label of ['Alpha', 'Beta', 'Gamma']) {
      const toggle = screen.getByRole('button', { name: label });
      expect(toggle.closest('h2')).not.toBeNull();
      expect(toggle).toHaveAttribute('aria-expanded', 'true');
    }
  });

  it('Edge: aria-controls resolves to the Section rows container', () => {
    const { container } = render(<Harness />);

    for (const label of ['Alpha', 'Beta', 'Gamma']) {
      const toggle = screen.getByRole('button', { name: label });
      const target = container.ownerDocument.getElementById(
        toggle.getAttribute('aria-controls') ?? '',
      );
      expect(target).not.toBeNull();
      expect(target).toHaveClass('lg-section__rows');
      expect(target?.closest('[data-section-key]')).toBe(toggle.closest('[data-section-key]'));
    }
  });

  it('Happy path: clicking collapses and expands, hiding the rows without unmounting them', async () => {
    const user = userEvent.setup();
    const { container } = render(<Harness />);
    const toggle = screen.getByRole('button', { name: 'Alpha' });
    const rows = () => container.querySelector('[data-section-key="alpha"] .lg-section__rows');

    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(rows()).toHaveAttribute('hidden');
    expect(rowKeys(rows() as HTMLElement)).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
    expect(screen.queryByRole('textbox', { name: 'One' })).not.toBeInTheDocument();

    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(rows()).not.toHaveAttribute('hidden');
  });

  it('Edge: a value typed in a collapsed Section survives collapse and expand', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const field = screen.getByRole('textbox', { name: 'One' });
    await user.clear(field);
    await user.type(field, '42');

    await user.click(screen.getByRole('button', { name: 'Alpha' }));
    await user.click(screen.getByRole('button', { name: 'Alpha' }));

    expect(screen.getByRole('textbox', { name: 'One' })).toBe(field);
    expect(field).toHaveValue('42');
  });

  it('Edge: a collapsed Section stays collapsed for keys the form does not know', () => {
    render(<Harness initial={['ghost', 'beta']} />);

    expect(screen.getByRole('button', { name: 'Beta' })).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByRole('button', { name: 'Alpha' })).toHaveAttribute('aria-expanded', 'true');
  });

  it('Edge: the changed badge shows only while collapsed and receives each drawn Row paths and key', () => {
    const badge = vi.fn((paths: readonly (readonly string[])[]) => (
      <span data-testid="badge">{paths.length}</span>
    ));
    const { rerender } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        collapsedSectionKeys={['beta']}
        onToggleSection={vi.fn()}
        sectionBadgeBuilder={badge}
      />,
    );

    expect(screen.getAllByTestId('badge')).toHaveLength(1);
    expect(screen.getByRole('button', { name: /Beta/ })).toContainElement(
      screen.getByTestId('badge'),
    );
    expect(badge).toHaveBeenCalledWith(
      [['beta.note'], ['beta.stats.str', 'beta.stats.dex'], ['beta.pickA', 'beta.pickB']],
      ['beta.note', 'beta.stats', expect.stringMatching(/^beta#/)],
    );

    rerender(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        collapsedSectionKeys={[]}
        onToggleSection={vi.fn()}
        sectionBadgeBuilder={badge}
      />,
    );
    expect(screen.queryByTestId('badge')).not.toBeInTheDocument();
  });

  it('Edge: a replaced Section frame receives the collapsed flag', () => {
    const seen: boolean[] = [];
    function Frame({ title, children, collapsed }: SectionFrameProps) {
      seen.push(collapsed);
      return (
        <section>
          {title}
          {children}
        </section>
      );
    }
    render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        collapsedSectionKeys={['alpha']}
        onToggleSection={vi.fn()}
        frames={{ Section: Frame }}
      />,
    );

    expect(seen).toEqual([true, false, false]);
  });

  it('Edge: clicking with no handler registered does not throw', () => {
    render(<SheetForm fields={ARRANGE_FIELDS} data={{}} onChanged={vi.fn()} />);

    expect(() => fireEvent.click(screen.getByRole('heading', { name: 'Alpha' }))).not.toThrow();
  });
});

describe('the pinned-only filter', () => {
  it('Edge: zero pins draws an empty tree', () => {
    const { container } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged([])}
        onlyPinned
      />,
    );

    expect(sectionKeys(container)).toEqual([]);
    expect(rowKeys(container)).toEqual([]);
  });

  it('Happy path: only Sections with pins are drawn, with only their pinned Rows, in pin order', () => {
    const { container } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['alpha.three', 'beta#pick', 'alpha.one'])}
        onlyPinned
      />,
    );

    expect(sectionKeys(container)).toEqual(['alpha', 'beta']);
    expect(rowKeys(container)).toEqual(['alpha.three', 'alpha.one', 'beta#pick']);
  });

  it('Edge: a Section whose Rows are all pinned renders like any other', () => {
    const { container } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['gamma.solo'])}
        onlyPinned
      />,
    );

    expect(sectionKeys(container)).toEqual(['gamma']);
    expect(rowKeys(container)).toEqual(['gamma.solo']);
    expect(screen.getByRole('heading', { name: 'Gamma' })).toBeInTheDocument();
  });

  it('Edge: no row key is drawn twice', () => {
    const { container } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['alpha.two', 'beta.note', 'gamma.solo', 'alpha.two'])}
        onlyPinned
      />,
    );

    const keys = rowKeys(container);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it('Edge: titles are plain text while the filter is on, even with a toggle handler and collapsed keys', () => {
    render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['alpha.one'])}
        collapsedSectionKeys={['alpha']}
        onToggleSection={vi.fn()}
        onlyPinned
      />,
    );

    expect(screen.getByRole('heading', { name: 'Alpha' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Alpha' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'One' })).toBeInTheDocument();
    expect(document.querySelector('.lg-section__rows[hidden]')).toBeNull();
  });

  it('Edge: table Rows made adjacent by the filter share exactly one column header', () => {
    const { container } = render(
      <SheetForm
        fields={TABLES}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['grid.first', 'grid.last'], TABLES)}
        onlyPinned
      />,
    );

    expect(rowKeys(container)).toEqual(['grid.first', 'grid.last']);
    expect(container.querySelectorAll('.lg-field-table__header')).toHaveLength(1);
  });

  it('Edge: a table Row alone keeps its own header', () => {
    const { container } = render(
      <SheetForm
        fields={TABLES}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['grid.last'], TABLES)}
        onlyPinned
      />,
    );

    expect(rowKeys(container)).toEqual(['grid.last']);
    expect(container.querySelectorAll('.lg-field-table__header')).toHaveLength(1);
  });

  it('Edge: the filter off draws every Section and Row, with sections as drawn before', () => {
    const { container } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={arranged(['alpha.one'])}
      />,
    );

    expect(sectionKeys(container)).toEqual(['alpha', 'beta', 'gamma']);
    expect(rowKeys(container)).toHaveLength(7);
  });

  it('Edge: the filter never hides Sections the arrangement predates (stale pins are ignored)', () => {
    const { container } = render(
      <SheetForm
        fields={ARRANGE_FIELDS}
        data={{}}
        onChanged={vi.fn()}
        arrangement={{ sectionOrder: ['ghost'], pinned: ['ghost.row', 'beta.note'] }}
        onlyPinned
      />,
    );

    expect(sectionKeys(container)).toEqual(['beta']);
    expect(rowKeys(container)).toEqual(['beta.note']);
  });
});
