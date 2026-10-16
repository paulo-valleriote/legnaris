import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { FieldSchema } from '../api/models/template';
import { message } from '../l10n/strings';
import {
  EXCLUSIVE_NONE_VALUE,
  SheetExclusiveControl,
  exclusiveValueIsOn,
} from '../engine/exclusive_control';
import type { SheetExclusiveGroup, SheetField } from '../engine/field_grouping';
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

function field(path: string, overrides: Partial<FieldSchema> = {}): SheetField {
  return { path, schema: schema(overrides) };
}

function group(id: string, options: SheetField[]): SheetExclusiveGroup {
  return { id, options };
}

describe('sheet exclusive control', () => {
  it('Edge: exclusiveValueIsOn treats only a non-zero number as on (ENG-16, ENG-18)', () => {
    expect(exclusiveValueIsOn(1)).toBe(true);
    expect(exclusiveValueIsOn(-1)).toBe(true);
    expect(exclusiveValueIsOn(0)).toBe(false);
    expect(exclusiveValueIsOn(true)).toBe(false);
    expect(exclusiveValueIsOn(null)).toBe(false);
    expect(exclusiveValueIsOn('1')).toBe(false);
  });

  it('Edge: a group whose members all default to 0 offers the none entry (ENG-16)', () => {
    render(
      <SheetExclusiveControl
        group={group('ability', [field('stats.stealth', { label: 'Stealth' })])}
        onSelect={vi.fn()}
      />,
    );

    const none = screen.getByRole('option', { name: message('engine.none') });
    expect(none).toHaveAttribute('value', EXCLUSIVE_NONE_VALUE);
  });

  it('Edge: a group with a non-zero default and no active member shows the none hint, not a blank (ENG-16)', () => {
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth', defaultValue: 1 }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
        onSelect={vi.fn()}
      />,
    );

    const select = screen.getByRole('combobox', { name: 'Ability' });
    const options = within(select).getAllByRole('option');
    const hint = options.find((option) => option.textContent === message('engine.none'));
    expect(hint).toBeDefined();
    expect(hint).toHaveAttribute('value', '');
    expect(hint).toBeDisabled();
    expect(options.some((option) => option.getAttribute('value') === EXCLUSIVE_NONE_VALUE)).toBe(
      false,
    );
  });

  it('Edge: a group with a non-zero default and an active member offers no none entry (ENG-16)', () => {
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth', defaultValue: 1 }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
        selectedPath="stats.stealth"
        onSelect={vi.fn()}
      />,
    );

    const select = screen.getByRole('combobox', { name: 'Ability' });
    expect(within(select).queryByRole('option', { name: message('engine.none') })).toBeNull();
  });

  it('Edge: with no selection handler the control is inert (ENG-17)', () => {
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth' }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
      />,
    );

    expect(screen.getByRole('combobox', { name: 'Ability' })).toBeDisabled();
  });

  it('Edge: a marked group tints the control (ENG-22)', () => {
    const { container } = render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth' }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
        mark="modified"
        onSelect={vi.fn()}
      />,
    );

    expect(container.querySelector(`.${FIELD_MARKED_CLASS}`)).not.toBeNull();
  });

  it('Happy path: three members render one single-choice control, never independent toggles (ENG-14)', () => {
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth', defaultValue: 1 }),
          field('stats.athletics', { label: 'Athletics' }),
          field('stats.arcana', { label: 'Arcana' }),
        ])}
        selectedPath="stats.stealth"
        onSelect={vi.fn()}
      />,
    );

    const select = screen.getByRole('combobox', { name: 'Ability' });
    expect(within(select).getAllByRole('option')).toHaveLength(3);
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0);
  });

  it('Happy path: an option is labeled by its optionLabel, else its display name (ENG-15)', () => {
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth', optionLabel: 'Stealth (trained)' }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
        selectedPath="stats.stealth"
        onSelect={vi.fn()}
      />,
    );

    const select = screen.getByRole('combobox', { name: 'Ability' });
    expect(within(select).getByRole('option', { name: 'Stealth (trained)' })).toBeInTheDocument();
    expect(within(select).getByRole('option', { name: 'Athletics' })).toBeInTheDocument();
  });

  it('Happy path: selecting a member reports that member dotted path (ENG-18)', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth' }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
        onSelect={onSelect}
      />,
    );

    await user.selectOptions(screen.getByRole('combobox', { name: 'Ability' }), 'stats.athletics');

    expect(onSelect).toHaveBeenCalledWith('stats.athletics');
  });

  it('Happy path: selecting the none entry reports the clear-group signal (ENG-18)', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(
      <SheetExclusiveControl
        group={group('ability', [
          field('stats.stealth', { label: 'Stealth' }),
          field('stats.athletics', { label: 'Athletics' }),
        ])}
        onSelect={onSelect}
      />,
    );

    await user.selectOptions(
      screen.getByRole('combobox', { name: 'Ability' }),
      EXCLUSIVE_NONE_VALUE,
    );

    expect(onSelect).toHaveBeenCalledWith(EXCLUSIVE_NONE_VALUE);
  });
});
