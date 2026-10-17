import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Field } from '../ui/field';

describe('field primitive (UIP-11…UIP-15)', () => {
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
});
