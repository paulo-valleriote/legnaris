import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import type { FieldNode, FieldSchema } from '../api/models/template';
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

/** A synthetic template exercising every field type plus one exclusive group (SC-002). */
const FIELDS: Record<string, FieldNode> = {
  identity: group(
    {
      characterName: leaf({ type: 'text', label: 'Name' }),
      level: leaf({ type: 'number', label: 'Level', defaultValue: 1 }),
    },
    { label: 'Identity' },
  ),
  flags: group({ isNpc: leaf({ type: 'bool', label: 'NPC' }) }, { label: 'Flags' }),
  config: group(
    { houseRules: leaf({ type: 'number', label: 'House rules' }) },
    { label: 'Config' },
  ),
  derived: group(
    { total: leaf({ type: 'number', label: 'Total', isEditable: false, formula: 'str + 1' }) },
    { label: 'Derived' },
  ),
  misc: group(
    {
      mystery: leaf({ type: 'fancy', label: 'Mystery' }),
      armorClassAbility: leaf({ type: 'number', label: '' }),
    },
    { label: 'Misc' },
  ),
  ability: group(
    {
      stealth: leaf({ type: 'number', label: 'Stealth', exclusiveGroup: 'ability' }),
      athletics: leaf({ type: 'number', label: 'Athletics', exclusiveGroup: 'ability' }),
    },
    { label: 'Ability' },
  ),
};

const DATA: Record<string, unknown> = {
  identity: { characterName: 'Aragorn', level: 3 },
  flags: { isNpc: false },
  config: { houseRules: 0 },
  derived: { total: 12 },
  misc: { mystery: 'x', armorClassAbility: 15 },
  ability: { stealth: 1, athletics: 0 },
};

describe('sheet form composition (ENG-24, ENG-25, ENG-26, SC-002)', () => {
  it('Edge: passing both control sources is a construction error, not a half-rendered form (ENG-25)', () => {
    expect(() =>
      render(
        <SheetForm
          fields={FIELDS}
          data={DATA}
          onChanged={vi.fn()}
          controlBuilder={() => null}
          exclusiveControlBuilder={() => null}
        />,
      ),
    ).toThrow();
  });

  it('Edge: passing neither control source is a construction error (ENG-25)', () => {
    expect(() => render(<SheetForm fields={FIELDS} />)).toThrow();
  });

  it('Edge: passing only one builder is a construction error (ENG-25)', () => {
    expect(() => render(<SheetForm fields={FIELDS} controlBuilder={() => null} />)).toThrow();
  });

  it('Happy path: a template with every field type renders one complete form, every label derived (SC-002)', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    expect(screen.getByRole('heading', { name: 'Identity' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Aragorn');
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('3');
    expect(screen.getByRole('checkbox', { name: 'NPC' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'House rules' })).not.toBeChecked();
    expect(screen.getByRole('status', { name: 'Total' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'Mystery' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Ability' })).toBeInTheDocument();
    expect(screen.getByText('Armor class ability')).toBeInTheDocument();
    expect(container.textContent).not.toContain('characterName');
  });

  it('Happy path: data mode resolves each value by dotted path and reports an edit as (path, value) (ENG-24)', () => {
    const onChanged = vi.fn();
    render(<SheetForm fields={FIELDS} data={DATA} onChanged={onChanged} />);

    fireEvent.change(screen.getByRole('textbox', { name: 'Level' }), {
      target: { value: '7' },
    });
    expect(onChanged).toHaveBeenCalledWith('identity.level', 7);

    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), {
      target: { value: 'Boromir' },
    });
    expect(onChanged).toHaveBeenCalledWith('identity.characterName', 'Boromir');
  });

  it('Happy path: data mode shows the active exclusive member without independent toggles (ENG-14)', () => {
    render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    const select = screen.getByRole('combobox', { name: 'Ability' });
    expect(select).toHaveValue('ability.stealth');
  });

  it('Happy path: builder mode consults the injected builders once per row and renders no default control (ENG-26)', () => {
    const controlBuilder = vi.fn((field: { path: string }) => (
      <span data-testid={`builder-${field.path}`} />
    ));
    const exclusiveControlBuilder = vi.fn(() => <span data-testid="builder-exclusive" />);

    render(
      <SheetForm
        fields={FIELDS}
        controlBuilder={controlBuilder}
        exclusiveControlBuilder={exclusiveControlBuilder}
      />,
    );

    expect(controlBuilder).toHaveBeenCalledTimes(7);
    expect(exclusiveControlBuilder).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByTestId('builder-exclusive')).toBeInTheDocument();
  });
});

describe('sheet form table rows (ENG-04, ENG-06)', () => {
  const tableFields: Record<string, FieldNode> = {
    stats: group(
      {
        dexterity: leaf({ type: 'number', label: 'Dexterity' }),
        constitution: leaf({ type: 'number', label: 'Constitution' }),
      },
      { label: 'Abilities', columns: ['dexterity', 'constitution'] },
    ),
  };

  it('Happy path: collapsed sections render an expand button instead of rows, and toggle reports the key (F3 progressive disclosure)', () => {
    const onToggle = vi.fn();
    render(
      <SheetForm
        fields={FIELDS}
        data={DATA}
        onChanged={vi.fn()}
        collapsedSectionKeys={['flags', 'config']}
        onToggleSection={onToggle}
      />,
    );

    expect(screen.getByRole('button', { name: 'Flags: Expand section' })).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'NPC' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Flags: Expand section' }));
    expect(onToggle).toHaveBeenCalledWith('flags');
  });

  it('Happy path: a declared-column group draws one row with a shared header and aligned cells (ENG-04, ENG-06)', () => {
    const { container } = render(
      <SheetForm
        fields={tableFields}
        data={{ stats: { dexterity: 14, constitution: 12 } }}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByRole('textbox', { name: 'Dexterity' })).toHaveValue('14');
    expect(container.querySelectorAll('.lg-field-table__header')).toHaveLength(1);
    expect(container.querySelectorAll('.lg-field-table__row')).toHaveLength(1);
    expect(container.querySelectorAll('.lg-field-table__cell--number')).toHaveLength(4);
  });
});
