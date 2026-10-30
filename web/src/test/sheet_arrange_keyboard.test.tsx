import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { ARRANGE_FRAMES, usePrefersReducedMotion } from '../sheet/arrange_frames';
import { createArrangeStub } from './arrange_stub';
import { renderApp } from './render';
import { SHEET_ID } from './sheet_fixtures';

const here = path.dirname(fileURLToPath(import.meta.url));
const SHEET_URL = `/sheets/${SHEET_ID}`;

function rect(top: number, height: number): DOMRect {
  return {
    x: 0,
    y: top,
    top,
    left: 0,
    right: 300,
    bottom: top + height,
    width: 300,
    height,
    toJSON: () => ({}),
  } as DOMRect;
}

const rowKeys = (scope: ParentNode = document) =>
  Array.from(scope.querySelectorAll<HTMLElement>('[data-row-key]')).map((e) => e.dataset.rowKey);

const sectionKeys = () =>
  Array.from(document.querySelectorAll<HTMLElement>('[data-section-key]')).map(
    (e) => e.dataset.sectionKey,
  );

/** jsdom has no layout: stack rows 100 apart and Sections 1000 apart so dnd-kit can measure. */
function stubLayout() {
  vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
    const rows = Array.from(document.querySelectorAll('[data-row-key]'));
    const sections = Array.from(document.querySelectorAll('[data-section-key]'));
    if (this.hasAttribute('data-row-key')) {
      return rect(rows.indexOf(this) * 100, 50);
    }
    if (this.hasAttribute('data-section-key')) {
      return rect(sections.indexOf(this) * 1000, 900);
    }
    return rect(0, 0);
  });
  Element.prototype.scrollTo = vi.fn() as unknown as typeof Element.prototype.scrollTo;
  Element.prototype.scrollIntoView = vi.fn();
}

describe('keyboard sensor', () => {
  beforeEach(stubLayout);
  afterEach(() => {
    vi.restoreAllMocks();
  });

  async function arrange() {
    const stub = createArrangeStub();
    const user = userEvent.setup();
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: 'http://api.test', transport: stub.transport }),
    });
    await screen.findByTestId('sheet-name');
    await user.click(screen.getByRole('button', { name: message('arrange.start') }));
    return { stub, user };
  }

  const alphaHandle = () =>
    screen.getByRole('button', { name: messageWith('arrange.handle', { label: 'Alpha' }) });
  const star = (key: string) =>
    within(document.querySelector(`[data-row-key="${key}"]`) as HTMLElement).getByRole('button', {
      name: /^Pin /,
    });

  it('Happy path: Space lifts a Section, the arrow moves it, Space drops it and the order is staged', async () => {
    const { user, stub } = await arrange();
    const handle = alphaHandle();
    handle.focus();

    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}');
    await user.keyboard(' ');

    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']));
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
    expect(stub.puts()).toHaveLength(0);
    expect(handle).toHaveFocus();
  });

  it('Happy path: a pointer drag of a Section handle reorders the Sections', async () => {
    await arrange();
    const handle = alphaHandle();

    fireEvent.mouseDown(handle, { button: 0, clientX: 10, clientY: 10 });
    fireEvent.mouseMove(document, { clientX: 10, clientY: 1050 });
    fireEvent.mouseMove(document, { clientX: 10, clientY: 1100 });
    fireEvent.mouseUp(document, { clientX: 10, clientY: 1100 });

    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']));
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
  });

  it('Edge: Escape cancels a lift and leaves the order untouched', async () => {
    const { user } = await arrange();
    alphaHandle().focus();

    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}');
    await user.keyboard('{Escape}');

    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });

  it('Edge: lifting and dropping in place stages nothing', async () => {
    const { user } = await arrange();
    alphaHandle().focus();

    await user.keyboard(' ');
    await user.keyboard(' ');

    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });

  it('Edge: moving the first Section up or the last Section down stays inside the list', async () => {
    const { user } = await arrange();
    alphaHandle().focus();
    await user.keyboard(' ');
    await user.keyboard('{ArrowUp}{ArrowUp}');
    await user.keyboard(' ');
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);

    const gamma = screen.getByRole('button', {
      name: messageWith('arrange.handle', { label: 'Gamma' }),
    });
    gamma.focus();
    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}{ArrowDown}');
    await user.keyboard(' ');
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Edge: a Section dragged across several places keeps its Rows and their pins', async () => {
    const { user } = await arrange();
    await user.click(star('alpha.three'));
    alphaHandle().focus();

    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}{ArrowDown}');
    await user.keyboard(' ');

    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'gamma', 'alpha']));
    expect(rowKeys(document.querySelector('[data-section-key="alpha"]') as HTMLElement)).toEqual([
      'alpha.three',
      'alpha.one',
      'alpha.two',
    ]);
    expect(star('alpha.three')).toHaveAttribute('aria-pressed', 'true');
  });

  it('Edge: a Row has no keyboard drag: Space on a star pins it and lifts nothing', async () => {
    const { user } = await arrange();
    star('alpha.two').focus();

    await user.keyboard(' ');

    expect(star('alpha.two')).toHaveAttribute('aria-pressed', 'true');
    expect(document.querySelector('[id^="DndLiveRegion"]')).not.toHaveTextContent(/Picked up/);
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Happy path: Done after a keyboard drag and a pin sends one PUT with both lists and no row_order', async () => {
    const { user, stub } = await arrange();
    await user.click(star('gamma.solo'));
    await user.click(star('alpha.two'));
    alphaHandle().focus();
    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}');
    await user.keyboard(' ');
    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']));

    await user.click(screen.getByRole('button', { name: message('arrange.done') }));

    await waitFor(() => expect(stub.puts()).toHaveLength(1));
    expect(stub.puts()[0].body).toBe(
      '{"layout":{"section_order":["beta","alpha","gamma"],"pinned":["gamma.solo","alpha.two"]}}',
    );
    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']));
  });

  it('Edge: Cancel after a drag restores the stored order and sends nothing', async () => {
    const { user, stub } = await arrange();
    alphaHandle().focus();
    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}');
    await user.keyboard(' ');
    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']));

    await user.click(screen.getByRole('button', { name: message('arrange.cancel') }));

    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(stub.puts()).toHaveLength(0);
  });

  it('Edge: Reset after a drag stages the Template order, which is no longer pending', async () => {
    const { user } = await arrange();
    alphaHandle().focus();
    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}');
    await user.keyboard(' ');
    await waitFor(() => expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']));

    await user.click(screen.getByRole('button', { name: message('arrange.reset') }));
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });

  it('Happy path: lifting announces the pick-up, the move and the cancel through the library live region', async () => {
    const { user } = await arrange();
    alphaHandle().focus();

    await user.keyboard(' ');

    await waitFor(() =>
      expect(document.querySelector('[id^="DndLiveRegion"]')).toHaveTextContent(
        messageWith('arrange.pickedUp', { label: 'Alpha', position: 1, count: 3 }),
      ),
    );
    await user.keyboard('{ArrowDown}');
    await waitFor(() =>
      expect(document.querySelector('[id^="DndLiveRegion"]')).toHaveTextContent(
        messageWith('arrange.position', { label: 'Alpha', position: 2, count: 3 }),
      ),
    );
    await user.keyboard('{Escape}');
    await waitFor(() =>
      expect(document.querySelector('[id^="DndLiveRegion"]')).toHaveTextContent(
        messageWith('arrange.dragCancelled', { label: 'Alpha' }),
      ),
    );
  });

  it('Happy path: dropping announces the final position', async () => {
    const { user } = await arrange();
    alphaHandle().focus();

    await user.keyboard(' ');
    await user.keyboard('{ArrowDown}');
    await user.keyboard(' ');

    await waitFor(() =>
      expect(document.querySelector('[id^="DndLiveRegion"]')).toHaveTextContent(
        messageWith('arrange.dropped', { label: 'Alpha', position: 2, count: 3 }),
      ),
    );
  });
});

describe('arrange frames', () => {
  it('Error: a frame refuses to render outside an ArrangeProvider', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const Root = ARRANGE_FRAMES.Root as NonNullable<typeof ARRANGE_FRAMES.Root>;

    expect(() =>
      render(
        <Root sectionKeys={[]}>
          <span />
        </Root>,
      ),
    ).toThrow(/ArrangeProvider/);
    vi.restoreAllMocks();
  });
});

describe('reduced motion', () => {
  function Probe() {
    return <p data-testid="probe">{String(usePrefersReducedMotion())}</p>;
  }

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('Happy path: the preference is read live from the media query', async () => {
    let matches = false;
    const listeners = new Set<() => void>();
    vi.spyOn(window, 'matchMedia').mockImplementation(
      () =>
        ({
          get matches() {
            return matches;
          },
          addEventListener: (_type: string, listener: () => void) => listeners.add(listener),
          removeEventListener: (_type: string, listener: () => void) => listeners.delete(listener),
        }) as unknown as MediaQueryList,
    );
    render(<Probe />);
    expect(screen.getByTestId('probe')).toHaveTextContent('false');

    matches = true;
    listeners.forEach((listener) => listener());

    await waitFor(() => expect(screen.getByTestId('probe')).toHaveTextContent('true'));
  });

  it('Edge: a browser without matchMedia reads as no preference', () => {
    const original = window.matchMedia;
    Object.defineProperty(window, 'matchMedia', {
      value: undefined,
      configurable: true,
      writable: true,
    });
    try {
      render(<Probe />);

      expect(screen.getByTestId('probe')).toHaveTextContent('false');
    } finally {
      Object.defineProperty(window, 'matchMedia', {
        value: original,
        configurable: true,
        writable: true,
      });
    }
  });

  it('Edge: every arrange transition is switched off under prefers-reduced-motion', () => {
    const css = readFileSync(path.resolve(here, '../sheet/sheet.css'), 'utf8');
    const reduced = css.slice(css.indexOf('@media (prefers-reduced-motion: reduce)'));

    expect(reduced).toContain('.lg-arrange-item--shifting');
    expect(reduced).toMatch(/transition:\s*none/);
    expect(css).not.toMatch(/\.lg-arrange[^{]*\{[^}]*animation/);
  });
});
