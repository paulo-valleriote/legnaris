import { useRef, useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Dialog } from '../ui/dialog';

describe('dialog primitive', () => {
  it('Happy path: exposes dialog role and title', () => {
    render(<Dialog open title="Confirm" onClose={() => {}} />);
    expect(screen.getByRole('dialog', { name: /Confirm/i })).toBeInTheDocument();
  });

  it('Happy path: the accessible name comes from the title element, not a label copy', () => {
    render(<Dialog open title="Confirm" onClose={() => {}} />);
    const dialog = screen.getByRole('dialog');
    const title = screen.getByRole('heading', { name: 'Confirm' });
    expect(dialog).not.toHaveAttribute('aria-label');
    expect(dialog).toHaveAttribute('aria-labelledby', title.id);
  });

  it('Edge: scrim activation closes dialog', async () => {
    const close = vi.fn();
    const { container } = render(<Dialog open title="X" onClose={close} />);
    fireEvent.click(container.querySelector('.lg-dialog__scrim')!);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('Happy path: Escape asks to close once', () => {
    const close = vi.fn();
    render(<Dialog open title="X" onClose={close} />);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('Happy path: actions render in the dialog footer', () => {
    render(
      <Dialog
        open
        title="Confirm"
        onClose={() => {}}
        actions={
          <>
            <button type="button">Cancel</button>
            <button type="button">Delete</button>
          </>
        }
      />,
    );
    const footer = screen.getByRole('dialog').querySelector('.lg-dialog__actions');
    expect(footer).not.toBeNull();
    expect(within(footer as HTMLElement).getByRole('button', { name: 'Delete' })).toBeVisible();
  });

  it('Happy path: a close label renders a named icon close button', async () => {
    const user = userEvent.setup();
    const close = vi.fn();
    const { container } = render(<Dialog open title="X" closeLabel="Close" onClose={close} />);
    expect(container.querySelector('.lg-dialog__close svg')).not.toBeNull();
    await user.click(screen.getByRole('button', { name: 'Close' }));
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('Happy path: the danger tone marks the panel', () => {
    const { container } = render(<Dialog open title="X" tone="danger" onClose={() => {}} />);
    expect(container.querySelector('.lg-dialog__panel--danger')).not.toBeNull();
  });

  it('Happy path: the focus trap wraps forward and backward inside the panel', () => {
    render(
      <Dialog open title="X" onClose={() => {}}>
        <button type="button">First</button>
        <button type="button">Last</button>
      </Dialog>,
    );
    const first = screen.getByRole('button', { name: 'First' });
    const last = screen.getByRole('button', { name: 'Last' });
    last.focus();
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Tab' });
    expect(first).toHaveFocus();
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Tab', shiftKey: true });
    expect(last).toHaveFocus();
  });

  it('Happy path: an initial focus target takes the focus instead of the panel', () => {
    function Harness() {
      const [ref] = useState(() => ({ current: null as HTMLButtonElement | null }));
      return (
        <Dialog open title="X" onClose={() => {}} initialFocusRef={ref}>
          <button type="button" ref={ref}>
            Inside
          </button>
        </Dialog>
      );
    }
    render(<Harness />);
    expect(screen.getByRole('button', { name: 'Inside' })).toHaveFocus();
  });

  it('Happy path: closing restores focus to the element that opened the dialog', async () => {
    const user = userEvent.setup();

    function Harness() {
      const [open, setOpen] = useState(false);
      const ref = useRef<HTMLButtonElement>(null);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>
            Open
          </button>
          <Dialog open={open} title="X" onClose={() => setOpen(false)} initialFocusRef={ref}>
            <button type="button" ref={ref}>
              Inside
            </button>
          </Dialog>
        </>
      );
    }

    render(<Harness />);
    const trigger = screen.getByRole('button', { name: 'Open' });
    await user.click(trigger);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it('Edge: a recreated onClose does not rerun the focus effect', () => {
    const { rerender } = render(
      <Dialog open title="X" onClose={() => {}}>
        <button type="button">Inside</button>
      </Dialog>,
    );
    const inside = screen.getByRole('button', { name: 'Inside' });
    inside.focus();
    rerender(
      <Dialog open title="X" onClose={() => {}}>
        <button type="button">Inside</button>
      </Dialog>,
    );
    expect(inside).toHaveFocus();
  });

  it('Happy path: an extra panel class is appended to the dialog panel', () => {
    const { container } = render(
      <Dialog open title="X" className="lg-test__panel" onClose={() => {}} />,
    );
    const panel = container.querySelector('.lg-dialog__panel');

    expect(panel).toHaveClass('lg-dialog__panel', 'lg-test__panel');
  });

  it('Happy path: a supplied titleId names the dialog and the title', () => {
    render(<Dialog open title="Terms" titleId="terms-title" onClose={() => {}} />);

    expect(screen.getByRole('dialog')).toHaveAttribute('aria-labelledby', 'terms-title');
    expect(screen.getByRole('heading', { name: 'Terms' })).toHaveAttribute('id', 'terms-title');
  });

  it('Happy path: an error slot sits above the actions as an alert', () => {
    const { container } = render(
      <Dialog
        open
        title="X"
        error="The server refused"
        onClose={() => {}}
        actions={<button type="button">Retry</button>}
      />,
    );
    const alert = screen.getByRole('alert');
    const actions = container.querySelector('.lg-dialog__actions');

    expect(alert).toHaveTextContent('The server refused');
    expect(alert).toHaveClass('lg-dialog__error');
    expect(alert.compareDocumentPosition(actions!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('Edge: Escape reaches only the topmost open dialog', () => {
    const outer = vi.fn();
    const inner = vi.fn();
    const { rerender } = render(
      <>
        <Dialog open title="Outer" onClose={outer} />
        <Dialog open title="Inner" onClose={inner} />
      </>,
    );

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(inner).toHaveBeenCalledTimes(1);
    expect(outer).not.toHaveBeenCalled();

    rerender(
      <>
        <Dialog open title="Outer" onClose={outer} />
        <Dialog open={false} title="Inner" onClose={inner} />
      </>,
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(outer).toHaveBeenCalledTimes(1);
    expect(inner).toHaveBeenCalledTimes(1);
  });
});
