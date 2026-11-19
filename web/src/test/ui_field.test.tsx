import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Field } from '../ui/field';

describe('field primitive', () => {
  it('Happy path: label is associated with control', () => {
    render(<Field label="Name" name="name" />);
    expect(screen.getByLabelText('Name')).toBeInTheDocument();
  });

  it('Edge: no error means no invalid marking or error association', () => {
    render(<Field label="X" />);
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'false');
  });

  it('Edge: disabled control is announced as disabled', () => {
    render(<Field label="X" disabled />);
    expect(screen.getByRole('textbox')).toBeDisabled();
  });

  it('Happy path: the error sits outside the label, so the name stays the label alone', () => {
    render(<Field label="Email" error="Enter a valid address" />);
    const input = screen.getByRole('textbox');
    const label = screen.getByText('Email').closest('label');
    expect(label).not.toBeNull();
    expect(label).not.toContainElement(screen.getByText('Enter a valid address'));
    expect(input).toHaveAccessibleName('Email');
  });

  it('Happy path: hint and error are both described, in that order', () => {
    render(<Field label="Email" hint="We send a code here." error="Required." />);
    const input = screen.getByRole('textbox');
    const describedBy = input.getAttribute('aria-describedby')!.split(' ');
    expect(describedBy).toHaveLength(2);
    expect(screen.getByText('We send a code here.')).toHaveAttribute('id', describedBy[0]);
    expect(screen.getByText('Required.')).toHaveAttribute('id', describedBy[1]);
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });

  it('Edge: only the hint is described when there is no error', () => {
    render(<Field label="Email" hint="We send a code here." />);
    const input = screen.getByRole('textbox');
    expect(input.getAttribute('aria-describedby')).toBe(
      screen.getByText('We send a code here.').id,
    );
  });

  it('Happy path: the input type, autocomplete, length and pattern are forwarded', () => {
    render(
      <Field
        label="Password"
        type="password"
        autoComplete="current-password"
        maxLength={64}
        pattern=".{8,}"
        required
      />,
    );
    const input = document.querySelector('input')!;
    expect(input).toHaveAttribute('type', 'password');
    expect(input).toHaveAttribute('autocomplete', 'current-password');
    expect(input).toHaveAttribute('maxlength', '64');
    expect(input).toHaveAttribute('pattern', '.{8,}');
    expect(input).toBeRequired();
  });

  it('Happy path: the numeric look is a class, never an inline style', () => {
    render(<Field label="Strength" numeric value="12" />);
    const input = screen.getByRole('textbox');
    expect(input.className).toContain('lg-form-field__input--numeric');
    expect(input.getAttribute('style')).toBeNull();
  });

  it('Happy path: a trailing control renders beside the input', () => {
    render(
      <Field
        label="Password"
        trailing={
          <button type="button" aria-label="Show password">
            Show
          </button>
        }
      />,
    );
    expect(screen.getByRole('button', { name: 'Show password' })).toBeInTheDocument();
  });

  it('Happy path: inputRef exposes the element and onBlur fires', () => {
    const onBlur = vi.fn();
    let element: HTMLInputElement | null = null;
    render(
      <Field
        label="Name"
        inputRef={(node) => {
          element = node;
        }}
        onBlur={onBlur}
      />,
    );
    const input = screen.getByRole('textbox');
    expect(element).toBe(input);
    fireEvent.blur(input);
    expect(onBlur).toHaveBeenCalledTimes(1);
  });

  it('Happy path: multiline renders a textarea with the same label, rows and styling', () => {
    render(<Field label="Concept" id="concept" multiline rows={4} value="A ranger" />);
    const textarea = screen.getByRole('textbox', { name: 'Concept' });

    expect(textarea.tagName).toBe('TEXTAREA');
    expect(textarea).toHaveAttribute('rows', '4');
    expect(textarea).toHaveValue('A ranger');
    expect(textarea).toHaveClass('lg-form-field__input', 'lg-form-field__textarea');
  });

  it('Happy path: a read-only field marks the control read-only', () => {
    render(<Field label="Share link" readOnly value="https://example.test/join/abc" />);

    expect(screen.getByRole('textbox', { name: 'Share link' })).toHaveAttribute('readonly');
  });

  it('Happy path: describedBy adds extra ids between the hint and the error', () => {
    render(
      <Field label="Concept" hint="An example." describedBy="extra-a extra-b" error="Too long." />,
    );
    const input = screen.getByRole('textbox');

    expect(input.getAttribute('aria-describedby')).toBe(
      `${screen.getByText('An example.').id} extra-a extra-b ${screen.getByText('Too long.').id}`,
    );
  });

  it('Happy path: the error is a polite live region so a new error is announced', () => {
    render(<Field label="Email" error="Enter a valid address" />);

    expect(screen.getByText('Enter a valid address')).toHaveAttribute('aria-live', 'polite');
  });

  it('Edge: a multiline field still renders its hint and error as siblings of the control', () => {
    render(<Field label="Concept" multiline hint="Hint" error="Error" />);
    const label = screen.getByText('Concept').closest('label');

    expect(label).not.toContainElement(screen.getByText('Hint'));
    expect(label).not.toContainElement(screen.getByText('Error'));
    expect(screen.getByRole('textbox', { name: 'Concept' })).toHaveAttribute(
      'aria-invalid',
      'true',
    );
  });
});
