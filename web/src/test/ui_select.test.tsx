import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Select } from '../ui/select';

const OPTIONS = [
  { value: 'dnd5e', label: 'Dungeons & Dragons 5e' },
  { value: 'pbta', label: 'Powered by the Apocalypse' },
];

describe('select primitive', () => {
  it('Happy path: label is associated and options render in order', () => {
    render(
      <Select
        label="Template"
        name="template"
        options={OPTIONS}
        value="dnd5e"
        onChange={() => undefined}
      />,
    );
    const select = screen.getByLabelText('Template');
    expect(select.tagName).toBe('SELECT');
    expect(screen.getAllByRole('option')).toHaveLength(2);
    expect(screen.getByRole('option', { name: 'Powered by the Apocalypse' })).toHaveAttribute(
      'value',
      'pbta',
    );
  });

  it('Happy path: hint and error are described and the error marks the control', () => {
    render(<Select label="Template" options={OPTIONS} hint="Choose a system." error="Pick one." />);
    const select = screen.getByLabelText('Template');
    expect(select).toHaveAttribute('aria-invalid', 'true');
    const ids = select.getAttribute('aria-describedby')!.split(' ');
    expect(ids).toHaveLength(2);
    expect(screen.getByText('Choose a system.')).toHaveAttribute('id', ids[0]);
    expect(screen.getByText('Pick one.')).toHaveAttribute('id', ids[1]);
  });

  it('Happy path: the control reuses the field styling classes', () => {
    render(<Select label="Template" options={OPTIONS} />);
    const select = screen.getByLabelText('Template');
    expect(select.className).toContain('lg-form-field__input');
    expect(select.className).toContain('lg-form-field__select');
  });

  it('Happy path: a change reports the chosen value', () => {
    const onChange = vi.fn();
    render(<Select label="Template" options={OPTIONS} value="dnd5e" onChange={onChange} />);
    fireEvent.change(screen.getByLabelText('Template'), { target: { value: 'pbta' } });
    expect(onChange).toHaveBeenCalledWith('pbta');
  });

  it('Edge: the accessible name is the label alone, never the hint or error', () => {
    render(<Select label="Template" options={OPTIONS} hint="Hint" error="Error" />);
    expect(screen.getByLabelText('Template')).toHaveAccessibleName('Template');
  });

  it('Edge: a disabled select is announced as disabled', () => {
    render(<Select label="Template" options={OPTIONS} disabled />);
    expect(screen.getByLabelText('Template')).toBeDisabled();
  });

  it('Happy path: describedBy adds extra ids and the error is a polite live region', () => {
    render(
      <Select
        label="Template"
        options={OPTIONS}
        hint="Choose a system."
        describedBy="kind-hint"
        error="Pick one."
      />,
    );
    const select = screen.getByLabelText('Template');

    expect(select.getAttribute('aria-describedby')).toBe(
      `${screen.getByText('Choose a system.').id} kind-hint ${screen.getByText('Pick one.').id}`,
    );
    expect(screen.getByText('Pick one.')).toHaveAttribute('aria-live', 'polite');
  });
});
