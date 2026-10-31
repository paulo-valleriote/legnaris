import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { FieldSchema } from '../api/models/template';
import { messageWith } from '../l10n/strings';
import { SheetFieldControl } from '../engine/field_control';

const NUMBER_SCHEMA: FieldSchema = {
  type: 'number',
  label: 'Dexterity',
  isEditable: true,
  defaultValue: 0,
};

function Harness({
  initial,
  onEmit,
}: {
  readonly initial: unknown;
  readonly onEmit: (value: unknown) => void;
}) {
  const [value, setValue] = useState<unknown>(initial);
  return (
    <>
      <SheetFieldControl
        path="stats.dexterity"
        schema={NUMBER_SCHEMA}
        value={value}
        onChanged={(next) => {
          onEmit(next);
          setValue(next);
        }}
      />
      <button type="button" onClick={() => setValue(9)}>
        reset
      </button>
    </>
  );
}

function setup(initial: unknown = 5) {
  const onEmit = vi.fn();
  render(<Harness initial={initial} onEmit={onEmit} />);
  const input = screen.getByRole<HTMLInputElement>('textbox', { name: 'Dexterity' });
  return { onEmit, input };
}

function noEmittedNaN(onEmit: ReturnType<typeof vi.fn>) {
  for (const [value] of onEmit.mock.calls) {
    expect(typeof value === 'number' ? Number.isFinite(value) : value === null).toBe(true);
  }
}

describe('numeric input: negative and in-progress text', () => {
  it('Edge: typing "-" keeps the text and emits nothing', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.type(input, '-');

    expect(input).toHaveValue('-');
    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: typing "-" then "3" emits -3 and shows "-3"', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.type(input, '-3');

    expect(input).toHaveValue('-3');
    expect(onEmit).toHaveBeenCalledTimes(1);
    expect(onEmit).toHaveBeenLastCalledWith(-3);
  });

  it('Edge: typing "-3" over a stored 10 ends as -3, never as 103', async () => {
    const { onEmit, input } = setup(10);
    await userEvent.clear(input);
    await userEvent.type(input, '-3');

    expect(input).toHaveValue('-3');
    expect(onEmit).toHaveBeenLastCalledWith(-3);
  });

  it('Edge: typing "-" at the front of a stored 3 emits -3', async () => {
    const { onEmit, input } = setup(3);
    await userEvent.type(input, '-', { initialSelectionStart: 0, initialSelectionEnd: 0 });

    expect(input).toHaveValue('-3');
    expect(onEmit).toHaveBeenLastCalledWith(-3);
  });

  it.each([
    ['Edge: a lone point', '.'],
    ['Edge: a minus and a point', '-.'],
    ['Edge: a lone minus', '-'],
  ])('%s is shown and emits nothing', (_name, typed) => {
    const { onEmit, input } = setup(5);
    fireEvent.change(input, { target: { value: typed } });

    expect(input).toHaveValue(typed);
    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: "-.5" resolves to -0.5 once the digit arrives', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.type(input, '-.5');

    expect(input).toHaveValue('-.5');
    expect(onEmit).toHaveBeenCalledTimes(1);
    expect(onEmit).toHaveBeenLastCalledWith(-0.5);
  });

  it('Edge: "-0" emits a positive zero and keeps the typed text', () => {
    const { onEmit, input } = setup(5);
    fireEvent.change(input, { target: { value: '-0' } });

    expect(input).toHaveValue('-0');
    expect(onEmit).toHaveBeenLastCalledWith(0);
    expect(Object.is(onEmit.mock.lastCall?.[0], -0)).toBe(false);
  });

  it('Edge: a trailing point is kept on screen while the number is emitted', () => {
    const { onEmit, input } = setup(null);
    fireEvent.change(input, { target: { value: '3.' } });

    expect(input).toHaveValue('3.');
    expect(onEmit).toHaveBeenLastCalledWith(3);
  });

  it('Edge: pasting "-3" emits -3', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.click(input);
    await userEvent.paste('-3');

    expect(input).toHaveValue('-3');
    expect(onEmit).toHaveBeenLastCalledWith(-3);
  });

  it.each([
    ['Edge: a double minus', '--3'],
    ['Edge: a trailing minus', '3-'],
    ['Edge: letters', 'abc'],
    ['Edge: an overflowing exponent', '1e999'],
    ['Edge: a minus overflowing exponent', '-1e999'],
    ['Edge: the word Infinity', 'Infinity'],
    ['Edge: a negative Infinity', '-Infinity'],
    ['Edge: NaN', 'NaN'],
    ['Edge: two points', '1.2.3'],
  ])('%s is rejected: stored value and shown text stay', (_name, typed) => {
    const { onEmit, input } = setup(5);
    fireEvent.change(input, { target: { value: typed } });

    expect(onEmit).not.toHaveBeenCalled();
    expect(input).toHaveValue('5');
  });

  it('Edge: a rejected keystroke after "-" keeps the pending "-"', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.type(input, '-');
    await userEvent.type(input, '-');

    expect(input).toHaveValue('-');
    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: clearing the field emits null, also from a negative number', async () => {
    const { onEmit, input } = setup(-4);
    expect(input).toHaveValue('-4');

    await userEvent.clear(input);

    expect(input).toHaveValue('');
    expect(onEmit).toHaveBeenLastCalledWith(null);
  });

  it('Edge: an external value change replaces a pending "-"', async () => {
    const { onEmit, input } = setup(5);
    await userEvent.type(input, '{Backspace}-');
    expect(input).toHaveValue('-');
    onEmit.mockClear();

    await userEvent.click(screen.getByRole('button', { name: 'reset' }));

    expect(input).toHaveValue('9');
    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: leaving the field with a pending "-" restores the last reported value', async () => {
    const { onEmit, input } = setup(5);
    await userEvent.clear(input);
    await userEvent.type(input, '-');
    expect(input).toHaveValue('-');

    await userEvent.tab();

    expect(input).toHaveValue('');
    expect(onEmit).toHaveBeenLastCalledWith(null);
  });

  it('Edge: leaving the field with a pending "-" over an unchanged value restores that value', async () => {
    const { onEmit, input } = setup(5);
    fireEvent.change(input, { target: { value: '-' } });
    expect(input).toHaveValue('-');

    fireEvent.blur(input);

    expect(input).toHaveValue('5');
    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: leaving the field with a resolved negative number keeps it', async () => {
    const { input } = setup(null);
    await userEvent.type(input, '-3');
    await userEvent.tab();

    expect(input).toHaveValue('-3');
  });

  it('Edge: an inert field draws no sign toggle', () => {
    render(<SheetFieldControl path="stats.dexterity" schema={NUMBER_SCHEMA} value={-3} />);

    expect(screen.getByRole('textbox', { name: 'Dexterity' })).toBeDisabled();
    expect(screen.queryByRole('button', { hidden: true })).not.toBeInTheDocument();
  });

  it('Edge: no emitted value is ever NaN across a messy typing session', async () => {
    const { onEmit, input } = setup(5);
    await userEvent.clear(input);
    await userEvent.type(input, '-.-3e9-x..5');

    noEmittedNaN(onEmit);
    expect(onEmit.mock.calls.length).toBeGreaterThan(0);
  });

  it('Happy path: typing -3.5 emits -3.5', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.type(input, '-3.5');

    expect(input).toHaveValue('-3.5');
    expect(onEmit).toHaveBeenLastCalledWith(-3.5);
    noEmittedNaN(onEmit);
  });

  it('Happy path: an external negative value is shown as typed text', () => {
    const { input } = setup(-12);

    expect(input).toHaveValue('-12');
  });
});

describe('numeric input: sign toggle for keyboards without a minus key', () => {
  const toggleName = messageWith('engine.toggleSign', { label: 'Dexterity' });

  it('Happy path: the toggle turns a stored 5 into -5 and back, keeping focus on the input', async () => {
    const { onEmit, input } = setup(5);
    await userEvent.click(input);
    const toggle = screen.getByRole('button', { name: toggleName });

    await userEvent.click(toggle);
    expect(input).toHaveValue('-5');
    expect(onEmit).toHaveBeenLastCalledWith(-5);
    expect(input).toHaveFocus();

    await userEvent.click(toggle);
    expect(input).toHaveValue('5');
    expect(onEmit).toHaveBeenLastCalledWith(5);
  });

  it('Edge: the toggle on an empty field shows a pending "-" and emits nothing', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.click(input);

    await userEvent.click(screen.getByRole('button', { name: toggleName }));

    expect(input).toHaveValue('-');
    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: the toggle keeps a decimal and a trailing point', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.click(input);
    fireEvent.change(input, { target: { value: '2.' } });
    onEmit.mockClear();

    await userEvent.click(screen.getByRole('button', { name: toggleName }));

    expect(input).toHaveValue('-2.');
    expect(onEmit).toHaveBeenLastCalledWith(-2);
  });

  it('Edge: the toggle is not a tab stop, so a keyboard user types the minus instead', () => {
    const { input } = setup(5);
    input.focus();

    expect(screen.getByRole('button', { name: toggleName })).toHaveAttribute('tabindex', '-1');
  });

  it('Edge: pressing the toggle does not take focus from the input', () => {
    const { input } = setup(5);
    input.focus();
    const toggle = screen.getByRole('button', { name: toggleName });

    const notPrevented = fireEvent.mouseDown(toggle);

    expect(notPrevented).toBe(false);
    expect(input).toHaveFocus();
  });
});
