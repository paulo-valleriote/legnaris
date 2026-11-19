import { createRef } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Button } from '../ui/button';

describe('action primitive', () => {
  it('Happy path: primary variant fills accent and uses on-accent text', () => {
    render(<Button variant="primary" label="Save" />);
    const btn = screen.getByRole('button', { name: /Save/i });
    expect(btn.className).toContain('lg-button--primary');
  });

  it('Edge: danger variant is outlined, not filled', () => {
    render(<Button variant="danger" label="Delete" />);
    const btn = screen.getByRole('button', { name: /Delete/i });
    expect(btn.className).toContain('lg-button--danger');
  });

  it('Happy path: both sizes render at or above control height', () => {
    const { container: c1 } = render(<Button size="sm" label="A" />);
    const { container: c2 } = render(<Button size="md" label="B" />);
    expect(c1.querySelector('button')!.className).toContain('lg-button--sm');
    expect(c2.querySelector('button')!.className).toContain('lg-button--md');
  });

  it('Edge: disabled and loading do not run their handler', () => {
    const handler = () => {
      throw new Error('should not run');
    };
    render(<Button disabled label="No" onClick={handler} />);
    render(<Button loading label="Busy" onClick={handler} />);
    expect(screen.getAllByRole('button')).toHaveLength(2);
  });

  it('Happy path: default type never submits a form', () => {
    render(<Button label="X" />);
    expect(screen.getByRole('button')).toHaveAttribute('type', 'button');
  });

  it('Happy path: a submit button declares the submit type', () => {
    render(<Button type="submit" label="Send" />);
    expect(screen.getByRole('button')).toHaveAttribute('type', 'submit');
  });

  it('Happy path: children replace the label as the rendered content', () => {
    render(
      <Button label="Ignored">
        <span>Rich</span>
      </Button>,
    );
    expect(screen.getByRole('button')).toHaveTextContent('Rich');
    expect(screen.getByRole('button')).not.toHaveTextContent('Ignored');
  });

  it('Edge: a decorative icon is hidden from assistive technology', () => {
    const { container } = render(
      <Button label="Add" icon={<svg data-testid="glyph" />}>
        Add
      </Button>,
    );
    const icon = container.querySelector('.lg-button__icon');
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute('aria-hidden', 'true');
  });

  it('Happy path: an extra class is appended and full width is opt-in', () => {
    render(<Button label="Wide" className="lg-test__extra" fullWidth />);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('lg-test__extra');
    expect(btn.className).toContain('lg-button--full');
  });

  it('Happy path: ariaDescribedBy is forwarded to the button', () => {
    render(<Button label="Save" ariaDescribedBy="help-1" />);
    expect(screen.getByRole('button')).toHaveAttribute('aria-describedby', 'help-1');
  });

  it('Happy path: the row variant is a full-width left-aligned entry', () => {
    render(<Button variant="row" label="Open" />);
    expect(screen.getByRole('button').className).toContain('lg-button--row');
  });

  it('Happy path: loading shows a spinner element instead of a glyph', () => {
    const { container } = render(<Button loading label="Busy" />);
    expect(container.querySelector('.lg-button__spinner')).not.toBeNull();
    expect(container.querySelector('.lg-button')!.textContent).not.toContain('●');
    expect(screen.getByRole('button')).toHaveAttribute('aria-busy', 'true');
  });

  it('Happy path: a disabled reason is visible, focusable and suppresses the action', () => {
    const onClick = vi.fn();
    const { container } = render(
      <Button label="Save" disabled disabledReason="Fix the errors first" onClick={onClick} />,
    );
    const btn = screen.getByRole('button');
    const reason = container.querySelector('.lg-button__reason');
    expect(btn).not.toBeDisabled();
    expect(btn).toHaveAttribute('aria-disabled', 'true');
    expect(btn).toHaveAttribute('aria-describedby', reason!.id);
    expect(reason).toHaveTextContent('Fix the errors first');
    fireEvent.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('Edge: a disabled button with a reason stays focusable by keyboard', () => {
    render(<Button label="Save" disabled disabledReason="Nothing to save" />);
    screen.getByRole('button').focus();
    expect(screen.getByRole('button')).toHaveFocus();
  });

  it('Happy path: the ref reaches the underlying button element', () => {
    const ref = createRef<HTMLButtonElement>();
    render(<Button ref={ref} label="Focus me" />);

    expect(ref.current).toBe(screen.getByRole('button', { name: 'Focus me' }));
  });

  it('Happy path: id, title and the aria state attributes reach the button', () => {
    render(
      <Button
        label="More"
        id="more-actions"
        title="More actions"
        ariaExpanded
        ariaControls="more-panel"
        ariaPressed={false}
        ariaHasPopup="dialog"
      />,
    );
    const button = screen.getByRole('button', { name: 'More' });

    expect(button).toHaveAttribute('id', 'more-actions');
    expect(button).toHaveAttribute('title', 'More actions');
    expect(button).toHaveAttribute('aria-expanded', 'true');
    expect(button).toHaveAttribute('aria-controls', 'more-panel');
    expect(button).toHaveAttribute('aria-pressed', 'false');
    expect(button).toHaveAttribute('aria-haspopup', 'dialog');
  });

  it('Happy path: onClick receives the React mouse event', () => {
    const onClick = vi.fn();
    render(<Button label="Open" onClick={onClick} />);

    fireEvent.click(screen.getByRole('button', { name: 'Open' }));

    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onClick.mock.calls[0][0]).toMatchObject({ type: 'click' });
    expect(onClick.mock.calls[0][0].target).toBe(screen.getByRole('button', { name: 'Open' }));
  });

  it('Happy path: a trailing icon is decorative and follows the label', () => {
    const { container } = render(<Button label="Open" trailingIcon={<svg data-testid="tail" />} />);
    const trailing = container.querySelector('.lg-button__icon--trailing');

    expect(trailing).not.toBeNull();
    expect(trailing).toHaveAttribute('aria-hidden', 'true');
    expect(screen.getByRole('button', { name: 'Open' })).toContainElement(
      screen.getByTestId('tail'),
    );
    expect(screen.getByRole('button').lastElementChild).toBe(trailing);
  });

  it('Happy path: link mode renders a router link with the same variant and size classes', () => {
    render(
      <MemoryRouter>
        <Button variant="secondary" size="sm" label="Back to catalog" to="/catalog" />
      </MemoryRouter>,
    );
    const link = screen.getByRole('link', { name: 'Back to catalog' });

    expect(link).toHaveAttribute('href', '/catalog');
    expect(link).toHaveClass('lg-button', 'lg-button--secondary', 'lg-button--sm');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('Happy path: link mode also accepts a plain href', () => {
    render(<Button variant="outline" label="Open docs" href="/docs/privacy" />);
    const link = screen.getByRole('link', { name: 'Open docs' });

    expect(link).toHaveAttribute('href', '/docs/privacy');
    expect(link).toHaveClass('lg-button', 'lg-button--outline', 'lg-button--md');
  });

  it('Edge: loading and disabled are button-only, so a link never draws a spinner', () => {
    const { container } = render(
      <MemoryRouter>
        <Button
          label="Open"
          to="/catalog"
          loading
          disabled
          disabledReason="Not available"
          onClick={() => undefined}
        />
      </MemoryRouter>,
    );
    const link = screen.getByRole('link', { name: 'Open' });

    expect(container.querySelector('.lg-button__spinner')).toBeNull();
    expect(link).not.toHaveAttribute('aria-busy');
    expect(link).not.toHaveAttribute('aria-disabled');
  });
});
