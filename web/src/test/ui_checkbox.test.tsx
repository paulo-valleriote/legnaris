import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Checkbox } from '../ui/checkbox';

describe('checkbox primitive', () => {
  it('Happy path: label visible and associated', () => {
    render(<Checkbox label="Agree" />);
    expect(screen.getByLabelText('Agree')).toBeInTheDocument();
  });
  it('Edge: non-boolean value not coerced', () => {
    render(<Checkbox label="A" checked={false} />);
    expect((screen.getByLabelText('A') as HTMLInputElement).checked).toBe(false);
  });
});
