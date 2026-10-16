import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { message } from '../l10n/strings';
import { SheetExclusiveControl } from '../engine/exclusive_control';
import { SheetFieldControl } from '../engine/field_control';
import type { SheetExclusiveGroup, SheetField } from '../engine/field_grouping';
import { markLabel, SheetForm } from '../engine/sheet_form';
import { FIELD_MARKED_CLASS } from '../engine/sheet_mark';

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return { type: 'number', label: '', isEditable: true, defaultValue: 0, ...overrides };
}

function leaf(overrides: Partial<FieldSchema> = {}): FieldNode {
  return { kind: 'leaf', schema: schema(overrides) };
}

function group(children: Record<string, FieldNode>, label: string): FieldNode {
  return { kind: 'group', children, label };
}

const FIELDS: Record<string, FieldNode> = {
  core: group(
    {
      strength: leaf({ label: 'Strength' }),
      stealth: leaf({ label: 'Stealth', exclusiveGroup: 'ability' }),
      athletics: leaf({ label: 'Athletics', exclusiveGroup: 'ability' }),
    },
    'Core',
  ),
};

describe('sheet marks (ENG-10, ENG-22, ENG-23)', () => {
  it('Edge: markLabel yields the modified and recalculated copy (ENG-22)', () => {
    expect(markLabel('modified')).toBe(message('engine.modified'));
    expect(markLabel('recalculated')).toBe(message('engine.recalculated'));
  });

  it('Edge: a derived marked control drops the Calculated label and tints even when empty (ENG-10, ENG-22)', () => {
    const { container } = render(
      <SheetFieldControl
        path="derived.total"
        schema={schema({ label: 'Total', isEditable: false, formula: 'str + 1' })}
        value={null}
        mark="modified"
      />,
    );

    expect(container.querySelector(`.${FIELD_MARKED_CLASS}`)).not.toBeNull();
    expect(screen.queryByText(message('engine.calculated'))).not.toBeInTheDocument();
  });

  it('Edge: a derived control without a mark still shows the Calculated label (ENG-10)', () => {
    render(
      <SheetFieldControl
        path="derived.total"
        schema={schema({ label: 'Total', isEditable: false })}
        value={null}
      />,
    );

    expect(screen.getByText(message('engine.calculated'))).toBeInTheDocument();
  });

  it('Edge: an editable control marked recalculated tints an empty value (ENG-22)', () => {
    render(
      <SheetFieldControl
        path="core.strength"
        schema={schema({ label: 'Strength' })}
        value={null}
        mark="recalculated"
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByRole('textbox', { name: 'Strength' })).toHaveClass(FIELD_MARKED_CLASS);
  });

  it('Happy path: a marked exclusive control is tinted (ENG-22)', () => {
    const ability: SheetField = {
      path: 'core.stealth',
      schema: schema({ label: 'Stealth', exclusiveGroup: 'ability' }),
    };
    const group_: SheetExclusiveGroup = {
      id: 'ability',
      options: [ability, { path: 'core.athletics', schema: schema({ label: 'Athletics' }) }],
    };

    render(<SheetExclusiveControl group={group_} selectedPath="core.stealth" mark="modified" />);

    expect(screen.getByRole('combobox')).toHaveClass(FIELD_MARKED_CLASS);
  });

  it('Happy path: a marked row shows the matching label through the row mark seam (ENG-22, ENG-23)', () => {
    render(
      <SheetForm
        fields={FIELDS}
        controlBuilder={() => <span />}
        exclusiveControlBuilder={() => <span />}
        rowMarkBuilder={() => <span>{markLabel('modified')}</span>}
      />,
    );

    expect(screen.getAllByText(message('engine.modified'))).toHaveLength(2);
  });

  it('Happy path: rowMarkBuilder receives the single leaf path and every exclusive member path (ENG-23)', () => {
    const rowMarkBuilder = vi.fn(() => <span />);

    render(
      <SheetForm
        fields={FIELDS}
        controlBuilder={() => <span />}
        exclusiveControlBuilder={() => <span />}
        rowMarkBuilder={rowMarkBuilder}
      />,
    );

    expect(rowMarkBuilder).toHaveBeenCalledWith(['core.strength']);
    expect(rowMarkBuilder).toHaveBeenCalledWith(['core.stealth', 'core.athletics']);
  });
});
