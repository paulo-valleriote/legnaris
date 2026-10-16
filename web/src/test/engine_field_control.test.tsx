import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import type { FieldSchema } from '../api/models/template';
import { message } from '../l10n/strings';
import { SheetFieldControl } from '../engine/field_control';
import { FIELD_MARKED_CLASS } from '../engine/sheet_mark';

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return {
    type: 'number',
    label: '',
    isEditable: true,
    defaultValue: 0,
    ...overrides,
  };
}

describe('sheet field control', () => {
  it('Edge: a numeric input rejects text that does not resolve to a finite number (ENG-12)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="stats.dexterity"
        schema={schema({ label: 'Dexterity' })}
        value={5}
        onChanged={onChanged}
      />,
    );
    const input = screen.getByRole('textbox', { name: 'Dexterity' });

    fireEvent.change(input, { target: { value: 'abc' } });
    expect(onChanged).not.toHaveBeenCalled();
    expect(input).toHaveValue('5');

    fireEvent.change(input, { target: { value: 'Infinity' } });
    expect(onChanged).not.toHaveBeenCalled();
    expect(input).toHaveValue('5');

    fireEvent.change(input, { target: { value: '1e999' } });
    expect(onChanged).not.toHaveBeenCalled();
    expect(input).toHaveValue('5');
  });

  it('Edge: an emptied numeric input reports a cleared value, not zero (ENG-12)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="stats.dexterity"
        schema={schema({ label: 'Dexterity' })}
        value={5}
        onChanged={onChanged}
      />,
    );

    fireEvent.change(screen.getByRole('textbox', { name: 'Dexterity' }), {
      target: { value: '' },
    });

    expect(onChanged).toHaveBeenCalledWith(null);
  });

  it('Edge: with no change handler the numeric input is inert (ENG-13)', () => {
    render(
      <SheetFieldControl
        path="stats.dexterity"
        schema={schema({ label: 'Dexterity' })}
        value={5}
      />,
    );

    expect(screen.getByRole('textbox', { name: 'Dexterity' })).toBeDisabled();
  });

  it('Edge: with no change handler the text input is inert (ENG-13)', () => {
    render(
      <SheetFieldControl
        path="identity.name"
        schema={schema({ type: 'text', label: 'Name' })}
        value=""
      />,
    );

    expect(screen.getByRole('textbox', { name: 'Name' })).toBeDisabled();
  });

  it('Edge: with no change handler the toggle is inert (ENG-13)', () => {
    render(
      <SheetFieldControl
        path="flags.isNpc"
        schema={schema({ type: 'bool', label: 'NPC' })}
        value={false}
      />,
    );

    expect(screen.getByRole('checkbox', { name: 'NPC' })).toBeDisabled();
  });

  it('Edge: an editable leaf of an unrecognized type renders the read-only value without an editor (ENG-11)', () => {
    render(
      <SheetFieldControl
        path="misc.mystery"
        schema={schema({ type: 'fancy', label: 'Mystery' })}
        value={7}
      />,
    );

    expect(screen.getByRole('status', { name: 'Mystery' })).toHaveTextContent('7');
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByText(message('engine.calculated'))).not.toBeInTheDocument();
  });

  it('Edge: a null stored value renders empty and a later edit reports a real value (ENG-09, ENG-13)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="stats.dexterity"
        schema={schema({ label: 'Dexterity' })}
        value={null}
        onChanged={onChanged}
      />,
    );
    const input = screen.getByRole('textbox', { name: 'Dexterity' });
    expect(input).toHaveValue('');

    fireEvent.change(input, { target: { value: '7' } });
    expect(onChanged).toHaveBeenCalledWith(7);
  });

  it('Edge: a derived leaf marked by a preview drops the Calculated label and tints (ENG-10, ENG-22)', () => {
    const { container } = render(
      <SheetFieldControl
        path="derived.total"
        schema={schema({ type: 'number', label: 'Total', isEditable: false, formula: 'a + b' })}
        value={12}
        mark="recalculated"
      />,
    );

    expect(screen.queryByText(message('engine.calculated'))).not.toBeInTheDocument();
    expect(container.querySelector(`.${FIELD_MARKED_CLASS}`)).not.toBeNull();
  });

  it('Happy path: an editable number leaf renders a numeric input and reports the parsed number (ENG-07, ENG-12)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="stats.level"
        schema={schema({ label: 'Level' })}
        value={3}
        onChanged={onChanged}
      />,
    );
    const input = screen.getByRole('textbox', { name: 'Level' });

    expect(input).toHaveAttribute('inputmode', 'decimal');
    expect(input).toHaveValue('3');

    fireEvent.change(input, { target: { value: '7.5' } });
    expect(onChanged).toHaveBeenCalledWith(7.5);
  });

  it('Happy path: an editable text leaf renders a text input and reports the typed text (ENG-07)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="identity.name"
        schema={schema({ type: 'text', label: 'Name' })}
        value="Aragorn"
        onChanged={onChanged}
      />,
    );
    const input = screen.getByRole('textbox', { name: 'Name' });
    expect(input).toHaveValue('Aragorn');

    fireEvent.change(input, { target: { value: 'Boromir' } });
    expect(onChanged).toHaveBeenCalledWith('Boromir');
  });

  it('Happy path: an unchecked bool leaf reports true when toggled (ENG-08)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="flags.isNpc"
        schema={schema({ type: 'bool', label: 'NPC' })}
        value={false}
        onChanged={onChanged}
      />,
    );
    const toggle = screen.getByRole('checkbox', { name: 'NPC' });
    expect(toggle).not.toBeChecked();

    fireEvent.click(toggle);
    expect(onChanged).toHaveBeenCalledWith(true);
  });

  it('Happy path: a checked bool leaf reports false when toggled (ENG-08)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="flags.isNpc"
        schema={schema({ type: 'bool', label: 'NPC' })}
        value={true}
        onChanged={onChanged}
      />,
    );
    const toggle = screen.getByRole('checkbox', { name: 'NPC' });
    expect(toggle).toBeChecked();

    fireEvent.click(toggle);
    expect(onChanged).toHaveBeenCalledWith(false);
  });

  it('Happy path: a config proxy leaf reports 1 and 0 (ENG-08)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="config.houseRules"
        schema={schema({ label: 'House rules' })}
        value={0}
        onChanged={onChanged}
      />,
    );
    fireEvent.click(screen.getByRole('checkbox', { name: 'House rules' }));
    expect(onChanged).toHaveBeenCalledWith(1);
  });

  it('Happy path: a config proxy leaf turned off reports 0 (ENG-08)', () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl
        path="config.houseRules"
        schema={schema({ label: 'House rules' })}
        value={1}
        onChanged={onChanged}
      />,
    );
    fireEvent.click(screen.getByRole('checkbox', { name: 'House rules' }));
    expect(onChanged).toHaveBeenCalledWith(0);
  });

  it('Happy path: a derived leaf shows the Calculated mark and its formula on hover (ENG-09, ENG-10)', () => {
    render(
      <SheetFieldControl
        path="derived.total"
        schema={schema({ type: 'number', label: 'Total', isEditable: false, formula: 'str + 1' })}
        value={12}
      />,
    );

    expect(screen.getByRole('status', { name: 'Total' })).toHaveTextContent('12');
    expect(screen.getByText(message('engine.calculated'))).toBeInTheDocument();
    expect(screen.getByTitle('str + 1')).toBeInTheDocument();
  });

  it('Edge: a derived leaf without a formula exposes no tooltip (ENG-10)', () => {
    const { container } = render(
      <SheetFieldControl
        path="derived.total"
        schema={schema({ type: 'number', label: 'Total', isEditable: false })}
        value={12}
      />,
    );

    expect(screen.getByText(message('engine.calculated'))).toBeInTheDocument();
    expect(container.querySelector('[title]')).toBeNull();
  });
});
