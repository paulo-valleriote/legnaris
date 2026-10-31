import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { FieldSchema } from '../api/models/template';
import { messageWith } from '../l10n/strings';
import { SheetFieldControl } from '../engine/field_control';

const NUMBER_SCHEMA: FieldSchema = {
  type: 'number',
  label: 'Hit points',
  isEditable: true,
  defaultValue: 0,
};

const DERIVED_SCHEMA: FieldSchema = { ...NUMBER_SCHEMA, isEditable: false, formula: 'a + b' };
const TEXT_SCHEMA: FieldSchema = { ...NUMBER_SCHEMA, type: 'text' };

const LABEL = 'Hit points';
const DECREASE = messageWith('engine.decrease', { label: LABEL });
const INCREASE = messageWith('engine.increase', { label: LABEL });

interface HarnessProps {
  readonly initial: unknown;
  readonly onEmit: (value: unknown) => void;
  readonly schema?: FieldSchema;
  readonly labelId?: string;
  readonly inert?: boolean;
}

function Harness({
  initial,
  onEmit,
  schema = NUMBER_SCHEMA,
  labelId = 'row-label',
  inert,
}: HarnessProps) {
  const [value, setValue] = useState<unknown>(initial);
  return (
    <SheetFieldControl
      path="stats.hp"
      schema={schema}
      value={value}
      labelId={labelId}
      onChanged={
        inert
          ? undefined
          : (next) => {
              onEmit(next);
              setValue(next);
            }
      }
    />
  );
}

function setup(initial: unknown = 5, extra: Partial<HarnessProps> = {}, focused = true) {
  const onEmit = vi.fn();
  render(<Harness initial={initial} onEmit={onEmit} {...extra} />);
  const input = screen.getByRole<HTMLInputElement>('textbox', { name: LABEL });
  if (focused && extra.inert !== true) {
    input.focus();
  }
  return { onEmit, input };
}

function stepper(name: string) {
  return screen.getByRole('button', { name, hidden: true });
}

function noButtons() {
  return screen.queryAllByRole('button', { hidden: true });
}

function everyEmitIsFiniteOrNull(onEmit: ReturnType<typeof vi.fn>) {
  for (const [value] of onEmit.mock.calls) {
    expect(typeof value === 'number' ? Number.isFinite(value) : value === null).toBe(true);
  }
}

describe('numeric input: steppers', () => {
  const rounding: ReadonlyArray<{
    name: string;
    initial: unknown;
    press: string;
    expected: number;
  }> = [
    { name: 'Edge: 1.5 plus one is 2.5', initial: 1.5, press: INCREASE, expected: 2.5 },
    { name: 'Edge: 0.1 plus one is 1.1', initial: 0.1, press: INCREASE, expected: 1.1 },
    {
      name: 'Edge: 0.7 minus one is -0.3, not -0.30000000000000004',
      initial: 0.7,
      press: DECREASE,
      expected: -0.3,
    },
    { name: 'Edge: 4.35 plus one is 5.35', initial: 4.35, press: INCREASE, expected: 5.35 },
    {
      name: 'Edge: 1 minus one is 0, never a negative zero',
      initial: 1,
      press: DECREASE,
      expected: 0,
    },
    { name: 'Edge: 0 minus one crosses to -1', initial: 0, press: DECREASE, expected: -1 },
    {
      name: 'Edge: -1 plus one crosses to 0, never a negative zero',
      initial: -1,
      press: INCREASE,
      expected: 0,
    },
    { name: 'Edge: -0.5 plus one is 0.5', initial: -0.5, press: INCREASE, expected: 0.5 },
    {
      name: 'Edge: a large value steps exactly',
      initial: 123456789012,
      press: INCREASE,
      expected: 123456789013,
    },
  ];

  it.each(rounding)('$name', async ({ initial, press, expected }) => {
    const { onEmit, input } = setup(initial);
    await userEvent.click(stepper(press));

    expect(onEmit).toHaveBeenCalledTimes(1);
    expect(Object.is(onEmit.mock.calls[0][0], expected)).toBe(true);
    expect(input).toHaveValue(String(expected));
  });

  it('Edge: 0.1 plus 0.2 style drift never reaches the draft after repeated steps', async () => {
    const { onEmit } = setup(0.1);
    for (let i = 0; i < 3; i += 1) {
      await userEvent.click(stepper(INCREASE));
    }

    expect(onEmit.mock.calls.map(([value]) => value)).toEqual([1.1, 2.1, 3.1]);
  });

  it('Edge: plus from a null value starts at zero and reports 1', async () => {
    const { onEmit, input } = setup(null);
    await userEvent.click(stepper(INCREASE));

    expect(onEmit).toHaveBeenLastCalledWith(1);
    expect(input).toHaveValue('1');
  });

  it('Edge: minus from an undefined value reports -1', async () => {
    const onEmit = vi.fn();
    render(<Harness initial={undefined} onEmit={onEmit} />);
    screen.getByRole('textbox', { name: LABEL }).focus();
    await userEvent.click(stepper(DECREASE));

    expect(onEmit).toHaveBeenLastCalledWith(-1);
    expect(screen.getByRole('textbox', { name: LABEL })).toHaveValue('-1');
  });

  it('Edge: plus from an emptied field reports 1', async () => {
    const { onEmit, input } = setup(7);
    await userEvent.clear(input);
    await userEvent.click(stepper(INCREASE));

    expect(onEmit).toHaveBeenLastCalledWith(1);
    expect(input).toHaveValue('1');
  });

  it.each([
    {
      name: 'Edge: a pending "-" then plus steps from zero to 1',
      typed: '-',
      press: INCREASE,
      expected: 1,
    },
    {
      name: 'Edge: a pending "-" then minus steps from zero to -1',
      typed: '-',
      press: DECREASE,
      expected: -1,
    },
    {
      name: 'Edge: a pending "." then plus steps from zero to 1',
      typed: '.',
      press: INCREASE,
      expected: 1,
    },
    {
      name: 'Edge: a pending "-." then minus steps from zero to -1',
      typed: '-.',
      press: DECREASE,
      expected: -1,
    },
  ])('$name', async ({ typed, press, expected }) => {
    const { onEmit, input } = setup(null);
    await userEvent.type(input, typed);
    expect(onEmit).not.toHaveBeenCalled();

    await userEvent.click(stepper(press));

    expect(onEmit).toHaveBeenCalledTimes(1);
    expect(onEmit).toHaveBeenLastCalledWith(expected);
    expect(input).toHaveValue(String(expected));
  });

  it('Edge: a value too large to tell from its successor is not stepped and nothing is reported', async () => {
    const { onEmit, input } = setup(9007199254740992);
    await userEvent.click(stepper(INCREASE));

    expect(onEmit).not.toHaveBeenCalled();
    expect(input).toHaveValue('9007199254740992');
  });

  it('Edge: the largest finite value is not stepped and no infinity is reported', async () => {
    const { onEmit } = setup(Number.MAX_VALUE);
    await userEvent.click(stepper(INCREASE));

    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: a value written in exponent form steps without NaN', async () => {
    const { onEmit } = setup(1e21);
    await userEvent.click(stepper(DECREASE));

    everyEmitIsFiniteOrNull(onEmit);
  });

  it('Edge: pressing a stepper keeps focus on the input', async () => {
    const { input } = setup(3);
    input.focus();
    await userEvent.click(stepper(INCREASE));

    expect(input).toHaveFocus();
  });

  it('Edge: the stepper buttons are not tab stops and mousedown does not move focus', () => {
    const { input } = setup(3);
    input.focus();

    for (const name of [DECREASE, INCREASE]) {
      expect(stepper(name)).toHaveAttribute('tabindex', '-1');
      expect(fireEvent.mouseDown(stepper(name))).toBe(false);
    }
    expect(input).toHaveFocus();
  });

  it('Edge: the steppers and the sign toggle coexist, each with its own name', () => {
    setup(3);

    expect(stepper(DECREASE)).toBeInTheDocument();
    expect(stepper(INCREASE)).toBeInTheDocument();
    expect(stepper(messageWith('engine.toggleSign', { label: LABEL }))).toBeInTheDocument();
  });

  it('Edge: a stepper after the sign toggle steps the toggled value', async () => {
    const { onEmit } = setup(5);
    await userEvent.click(stepper(messageWith('engine.toggleSign', { label: LABEL })));
    await userEvent.click(stepper(INCREASE));

    expect(onEmit.mock.calls.map(([value]) => value)).toEqual([-5, -4]);
  });

  it('Edge: the stepper icons are decorative inline SVG on currentColor', () => {
    setup(3);

    for (const name of [DECREASE, INCREASE]) {
      const svg = stepper(name).querySelector('svg');
      expect(svg).toHaveAttribute('aria-hidden', 'true');
      expect(svg).not.toHaveAttribute('fill', expect.stringMatching(/#|rgb/));
    }
  });

  it('Edge: ArrowUp and ArrowDown step by one and report through the same path', async () => {
    const { onEmit, input } = setup(10);
    input.focus();
    await userEvent.keyboard('{ArrowUp}{ArrowUp}{ArrowDown}');

    expect(onEmit.mock.calls.map(([value]) => value)).toEqual([11, 12, 11]);
  });

  it('Edge: other keys do not step', async () => {
    const { onEmit, input } = setup(10);
    input.focus();
    await userEvent.keyboard('{ArrowLeft}{Home}{End}');

    expect(onEmit).not.toHaveBeenCalled();
  });

  it('Edge: a blur after a stepper keeps the stepped value', async () => {
    const { input } = setup(null);
    await userEvent.type(input, '-');
    await userEvent.click(stepper(INCREASE));
    input.blur();

    expect(input).toHaveValue('1');
  });
});

describe('numeric input: no steppers where they do not belong', () => {
  it('Edge: an inert field renders no stepper and no sign toggle', () => {
    setup(5, { inert: true });

    expect(noButtons()).toHaveLength(0);
  });

  it('Edge: a derived leaf renders no stepper', () => {
    render(<Harness initial={9} onEmit={vi.fn()} schema={DERIVED_SCHEMA} />);

    expect(screen.queryByRole('button', { name: INCREASE, hidden: true })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: DECREASE, hidden: true })).not.toBeInTheDocument();
  });

  it('Edge: a text leaf renders no stepper', () => {
    render(<Harness initial="abc" onEmit={vi.fn()} schema={TEXT_SCHEMA} />);

    expect(noButtons()).toHaveLength(0);
  });

  it('Edge: a table cell (no row label) keeps the sign toggle and has no steppers', () => {
    render(
      <SheetFieldControl path="stats.hp" schema={NUMBER_SCHEMA} value={3} onChanged={vi.fn()} />,
    );

    screen.getByRole('textbox', { name: LABEL }).focus();

    expect(screen.queryByRole('button', { name: INCREASE, hidden: true })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: DECREASE, hidden: true })).not.toBeInTheDocument();
    expect(stepper(messageWith('engine.toggleSign', { label: LABEL }))).toBeInTheDocument();
  });

  it('Edge: arrow keys still step in a table cell, the keyboard path of a field without buttons', async () => {
    const onChanged = vi.fn();
    render(
      <SheetFieldControl path="stats.hp" schema={NUMBER_SCHEMA} value={3} onChanged={onChanged} />,
    );
    screen.getByRole('textbox', { name: LABEL }).focus();
    await userEvent.keyboard('{ArrowUp}');

    expect(onChanged).toHaveBeenLastCalledWith(4);
  });

  it('Edge: arrow keys do nothing on an inert field', async () => {
    const { input } = setup(5, { inert: true });
    await userEvent.type(input, '{ArrowUp}');

    expect(input).toHaveValue('5');
  });
});

describe('numeric input: select on focus', () => {
  it('Edge: focusing selects the whole text', async () => {
    const { input } = setup(42, {}, false);
    input.focus();

    await waitFor(() => {
      expect(input.selectionStart).toBe(0);
      expect(input.selectionEnd).toBe(2);
    });
  });

  it('Edge: focusing an empty field selects nothing and does not throw', async () => {
    const { input } = setup(null, {}, false);
    input.focus();

    await waitFor(() => expect(input).toHaveFocus());
    expect(input.selectionStart).toBe(0);
    expect(input.selectionEnd).toBe(0);
  });

  it('Edge: focus selects the full range, so the next keystroke replaces the value', async () => {
    const { onEmit, input } = setup(42, {}, false);
    input.focus();
    await waitFor(() => expect(input.selectionStart).toBe(0));
    expect(input.selectionEnd).toBe(2);
    fireEvent.change(input, { target: { value: '7' } });

    expect(input).toHaveValue('7');
    expect(onEmit).toHaveBeenLastCalledWith(7);
  });

  it('Edge: a second tap on the focused field keeps the caret where it was placed', async () => {
    const { input } = setup(42, {}, false);
    input.focus();
    await waitFor(() => expect(input.selectionStart).toBe(0));
    input.setSelectionRange(1, 1);
    fireEvent.mouseDown(input);
    fireEvent.mouseUp(input);
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));

    expect(input.selectionStart).toBe(1);
    expect(input.selectionEnd).toBe(1);
  });

  it('Edge: a field blurred before the frame runs is not selected', async () => {
    const { input } = setup(42, {}, false);
    input.focus();
    input.blur();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));

    expect(input.selectionStart).toBe(input.selectionEnd);
  });

  it('Edge: text typed before the frame runs is not selected over by the late selection', async () => {
    const { input } = setup(null, {}, false);
    input.focus();
    fireEvent.change(input, { target: { value: '-' } });
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));

    expect(input).toHaveValue('-');
    expect(input.selectionStart).toBe(input.selectionEnd);
  });

  it('Edge: an inert field takes no focus and selects nothing', async () => {
    const { input } = setup(42, { inert: true }, false);
    await userEvent.click(input);
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));

    expect(input).toBeDisabled();
    expect(input).not.toHaveFocus();
    expect(input.selectionStart).toBe(input.selectionEnd);
  });

  it('Edge: a pending text is selected on refocus and replaced by the next key', async () => {
    const { onEmit, input } = setup(null, {}, false);
    await userEvent.type(input, '-');
    input.blur();
    input.focus();
    await userEvent.keyboard('4');

    expect(onEmit).toHaveBeenLastCalledWith(4);
  });
});

describe('numeric input: phone editing happy path', () => {
  it('Happy path: one tap on plus raises the value by one, one tap on minus lowers it', async () => {
    const { onEmit, input } = setup(30);
    await userEvent.click(stepper(INCREASE));
    await userEvent.click(stepper(DECREASE));
    await userEvent.click(stepper(DECREASE));

    expect(onEmit.mock.calls.map(([value]) => value)).toEqual([31, 30, 29]);
    expect(input).toHaveValue('29');
  });

  it('Happy path: both steppers are named after the field', () => {
    setup(30);

    expect(stepper('Decrease Hit points')).toBeInTheDocument();
    expect(stepper('Increase Hit points')).toBeInTheDocument();
  });

  it('Happy path: tap, type a new value, then step from it', async () => {
    const user = userEvent.setup();
    const { onEmit, input } = setup(42, {}, false);
    input.focus();
    await waitFor(() => expect(input.selectionStart).toBe(0));
    fireEvent.change(input, { target: { value: '18' } });
    await user.click(stepper(INCREASE));

    expect(onEmit.mock.calls.map(([value]) => value)).toEqual([18, 19]);
    everyEmitIsFiniteOrNull(onEmit);
  });
});
