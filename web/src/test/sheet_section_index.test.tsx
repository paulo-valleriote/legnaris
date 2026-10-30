import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import { message, messageWith } from '../l10n/strings';
import { ArrangeProvider } from '../sheet/arrange_frames';
import type { SectionLink } from '../sheet/section_index';
import { SectionIndex } from '../sheet/section_index';
import { createLayoutStore } from '../sheet/layout_store';
import { createApiClient } from '../api/client';
import { createMemorySheetStore } from '../library/sheet_store';
import { createFakeTransport, successOutcome } from './fake_transport';
import { arrangeSections } from './layout_fixtures';

const LINKS: SectionLink[] = [
  { key: 'identity', label: 'Identity' },
  { key: 'stats', label: 'Stats' },
  { key: 'skills', label: 'Skills' },
];

class StubObserver {
  static instances: StubObserver[] = [];
  readonly observed = new Set<Element>();
  disconnected = false;
  constructor(
    readonly callback: IntersectionObserverCallback,
    readonly options: IntersectionObserverInit | undefined,
  ) {
    StubObserver.instances.push(this);
  }
  observe(element: Element) {
    this.observed.add(element);
  }
  unobserve(element: Element) {
    this.observed.delete(element);
  }
  disconnect() {
    this.disconnected = true;
    this.observed.clear();
  }
  takeRecords() {
    return [];
  }
  trigger(entries: { target: Element; isIntersecting: boolean }[]) {
    this.callback(entries as unknown as IntersectionObserverEntry[], this as never);
  }
}

function placeTop(element: Element, top: number): void {
  element.getBoundingClientRect = () => ({
    top,
    bottom: top + 100,
    left: 0,
    right: 0,
    width: 0,
    height: 100,
    x: 0,
    y: top,
    toJSON: () => ({}),
  });
}

const latest = () => StubObserver.instances[StubObserver.instances.length - 1];

type IndexProps = ComponentProps<typeof SectionIndex>;

function props(overrides: Partial<IndexProps> = {}): IndexProps {
  return {
    sheetName: 'Aragorn',
    onTop: vi.fn(),
    sections: LINKS,
    onlyPinned: false,
    filterDisabled: false,
    pinnedCount: 0,
    onSetOnlyPinned: vi.fn(),
    onJump: vi.fn(),
    onEditPins: vi.fn(),
    epoch: 'play',
    ...overrides,
  };
}

function mount(overrides: Partial<IndexProps> = {}) {
  const merged = props(overrides);
  const view = render(
    <div className="lg-content">
      <div className="lg-sheet">
        <SectionIndex {...merged} />
        {merged.sections.map((section) => (
          <section key={section.key} data-section-key={section.key}>
            {section.label}
          </section>
        ))}
      </div>
    </div>,
  );
  return { ...view, props: merged };
}

const chip = (label: string | RegExp) =>
  within(screen.getByRole('navigation', { name: 'Sections' })).getByRole('button', {
    name: typeof label === 'string' ? messageWith('sheet.sectionLink', { label }) : label,
  });

beforeEach(() => {
  StubObserver.instances = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('the section links', () => {
  it('Edge: no Section draws no links, only the filter', () => {
    mount({ sections: [] });

    expect(screen.queryByRole('navigation', { name: 'Sections' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.pinnedOnly') })).toBeInTheDocument();
  });

  it('Edge: one Section draws no links, only the filter', () => {
    mount({ sections: [LINKS[0]] });

    expect(screen.queryByRole('navigation', { name: 'Sections' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.pinnedOnly') })).toBeInTheDocument();
  });

  it('Happy path: the links are plain buttons, not tabs, in the order given', () => {
    mount();

    const nav = screen.getByRole('navigation', { name: 'Sections' });
    expect(
      within(nav)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Identity', 'Stats', 'Skills']);
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  });

  it('Edge: the links follow a reordered list', () => {
    mount({ sections: [LINKS[2], LINKS[0], LINKS[1]] });

    const nav = screen.getByRole('navigation', { name: 'Sections' });
    expect(
      within(nav)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Skills', 'Identity', 'Stats']);
  });

  it('Edge: a long label keeps its full accessible name', () => {
    const long = 'An exceedingly long section label that the strip will ellipsise';
    mount({ sections: [LINKS[0], { key: 'long', label: long }] });

    expect(chip(long)).toHaveTextContent(long);
  });

  it('Edge: the whole row is one tab stop', () => {
    mount();

    const nav = screen.getByRole('navigation', { name: 'Sections' });
    const stops = within(nav)
      .getAllByRole('button')
      .filter((button) => button.getAttribute('tabindex') === '0');
    expect(stops).toHaveLength(1);
    expect(stops[0]).toHaveTextContent('Identity');
  });

  it('Edge: Tab goes filter, sheet name, then enters the links once and leaves them for the next control', async () => {
    const user = userEvent.setup();
    mount();
    screen.getByRole('button', { name: message('sheet.pinnedOnly') }).focus();

    await user.tab();
    expect(
      screen.getByRole('button', { name: messageWith('sheet.backToTop', { name: 'Aragorn' }) }),
    ).toHaveFocus();
    await user.tab();
    expect(chip('Identity')).toHaveFocus();
    await user.tab();
    expect(document.body).toHaveFocus();
  });

  it('Happy path: arrows, Home and End move focus and the tab stop, wrapping at the ends', async () => {
    const user = userEvent.setup();
    mount();
    chip('Identity').focus();

    await user.keyboard('{ArrowRight}');
    expect(chip('Stats')).toHaveFocus();
    expect(chip('Stats')).toHaveAttribute('tabindex', '0');
    expect(chip('Identity')).toHaveAttribute('tabindex', '-1');

    await user.keyboard('{End}');
    expect(chip('Skills')).toHaveFocus();
    await user.keyboard('{ArrowRight}');
    expect(chip('Identity')).toHaveFocus();
    await user.keyboard('{ArrowLeft}');
    expect(chip('Skills')).toHaveFocus();
    await user.keyboard('{Home}');
    expect(chip('Identity')).toHaveFocus();
  });

  it('Edge: other keys do nothing', async () => {
    const user = userEvent.setup();
    mount();
    chip('Identity').focus();

    await user.keyboard('{ArrowDown}a');

    expect(chip('Identity')).toHaveFocus();
  });

  it('Happy path: Enter and Space jump to the link Section; a click does too', async () => {
    const user = userEvent.setup();
    const { props: merged } = mount();
    chip('Stats').focus();

    await user.keyboard('{Enter}');
    await user.keyboard(' ');
    await user.click(chip('Skills'));

    expect(merged.onJump).toHaveBeenNthCalledWith(1, 'stats');
    expect(merged.onJump).toHaveBeenNthCalledWith(2, 'stats');
    expect(merged.onJump).toHaveBeenNthCalledWith(3, 'skills');
  });
});

describe('the trailing control', () => {
  it('Edge: without one the strip draws no trailing slot', () => {
    mount();

    expect(document.querySelector('.lg-sectionbar__trailing')).toBeNull();
  });

  it('Happy path: a given control sits in the strip after the links and stays reachable by Tab', async () => {
    const onClick = vi.fn();
    const user = userEvent.setup();
    mount({ trailing: <button onClick={onClick}>Extra</button> });

    const trailing = document.querySelector('.lg-sectionbar__trailing') as HTMLElement;
    const nav = screen.getByRole('navigation', { name: 'Sections' });
    expect(nav.compareDocumentPosition(trailing) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    await user.click(within(trailing).getByRole('button', { name: 'Extra' }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});

describe('the scrollspy', () => {
  it('Edge: without an IntersectionObserver no link is current and nothing throws', () => {
    vi.stubGlobal('IntersectionObserver', undefined);

    expect(() => mount()).not.toThrow();

    expect(document.querySelector('[aria-current]')).toBeNull();
  });

  it('Happy path: the last Section whose top reached the strip is current, with aria-current location', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    mount();
    const sections = (key: string) => document.querySelector(`[data-section-key="${key}"]`)!;
    placeTop(sections('stats'), -40);
    placeTop(sections('skills'), 400);

    expect(chip('Identity')).toHaveAttribute('aria-current', 'location');
    act(() =>
      latest().trigger([
        { target: sections('stats'), isIntersecting: true },
        { target: sections('skills'), isIntersecting: true },
      ]),
    );

    expect(chip('Stats')).toHaveAttribute('aria-current', 'location');
    expect(chip('Skills')).not.toHaveAttribute('aria-current');
    expect(document.querySelectorAll('[aria-current="location"]')).toHaveLength(1);
  });

  it('Edge: when no Section in the band has reached the strip yet, the first one in the band is current', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    mount();
    const sections = (key: string) => document.querySelector(`[data-section-key="${key}"]`)!;
    placeTop(sections('stats'), 300);
    placeTop(sections('skills'), 500);

    act(() =>
      latest().trigger([
        { target: sections('skills'), isIntersecting: true },
        { target: sections('stats'), isIntersecting: true },
      ]),
    );

    expect(chip('Stats')).toHaveAttribute('aria-current', 'location');
  });

  it('Edge: the observer root margin is the strip height at the top', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    mount();

    expect(latest().options?.rootMargin).toMatch(/^-\d+px 0% -\d+% 0%$/);
    expect(latest().observed.size).toBe(3);
  });

  it('Edge: at the end of the scroll the last Section is current', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    mount();
    const scroller = document.querySelector('.lg-content') as HTMLElement;
    Object.defineProperty(scroller, 'scrollHeight', { value: 1000, configurable: true });
    Object.defineProperty(scroller, 'clientHeight', { value: 500, configurable: true });
    Object.defineProperty(scroller, 'scrollTop', { value: 500, configurable: true });

    act(() => {
      fireEvent.scroll(scroller);
    });

    expect(chip('Skills')).toHaveAttribute('aria-current', 'location');
  });

  it('Edge: a region that does not scroll never counts as the end', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    mount();

    expect(chip('Identity')).toHaveAttribute('aria-current', 'location');
  });

  it('Edge: leaving the band with nothing else in it keeps the previous Section', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    mount();
    const stats = document.querySelector('[data-section-key="stats"]')!;
    act(() => latest().trigger([{ target: stats, isIntersecting: true }]));

    act(() => latest().trigger([{ target: stats, isIntersecting: false }]));

    expect(chip('Stats')).toHaveAttribute('aria-current', 'location');
  });

  it('Edge: a jump marks its Section current at once, before any scroll', async () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    const user = userEvent.setup();
    mount();
    const identity = document.querySelector('[data-section-key="identity"]')!;

    await user.click(chip('Skills'));
    act(() => latest().trigger([{ target: identity, isIntersecting: true }]));

    expect(chip('Skills')).toHaveAttribute('aria-current', 'location');
  });

  it('Edge: the observer is rebuilt when the drawn Sections change and the old one disconnects', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    const { rerender, props: merged } = mount();
    const first = latest();

    rerender(
      <div className="lg-content">
        <div className="lg-sheet">
          <SectionIndex {...merged} sections={[LINKS[0], LINKS[1]]} />
          <section data-section-key="identity" />
          <section data-section-key="stats" />
        </div>
      </div>,
    );

    expect(first.disconnected).toBe(true);
    expect(latest()).not.toBe(first);
    expect(latest().observed.size).toBe(2);
  });
});

describe('the pinned filter toggle', () => {
  it('Happy path: it is a toggle button whose click asks for the opposite state', async () => {
    const user = userEvent.setup();
    const { props: merged } = mount();

    const toggle = screen.getByRole('button', { name: message('sheet.pinnedOnly') });
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    await user.click(toggle);

    expect(merged.onSetOnlyPinned).toHaveBeenCalledWith(true);
  });

  it('Edge: pressed reads from the state and a second click asks for off', async () => {
    const user = userEvent.setup();
    const { props: merged } = mount({ onlyPinned: true, pinnedCount: 2 });

    const toggle = screen.getByRole('button', { name: message('sheet.pinnedOnly') });
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    await user.click(toggle);

    expect(merged.onSetOnlyPinned).toHaveBeenCalledWith(false);
  });

  it('Edge: while arranging the filter is disabled and reads as off even if the state is on', () => {
    mount({ onlyPinned: true, filterDisabled: true, pinnedCount: 2 });

    const toggle = screen.getByRole('button', { name: message('sheet.pinnedOnly') });
    expect(toggle).toBeDisabled();
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    expect(
      screen.queryByRole('button', { name: message('sheet.editPins') }),
    ).not.toBeInTheDocument();
  });

  const editPinsCases: {
    name: string;
    overrides: Partial<IndexProps>;
    shown: boolean;
  }[] = [
    {
      name: 'Edge: Edit pins is absent while the filter is off',
      overrides: { pinnedCount: 3 },
      shown: false,
    },
    {
      name: 'Edge: Edit pins is absent when the filter is on but nothing is pinned',
      overrides: { onlyPinned: true, pinnedCount: 0 },
      shown: false,
    },
    {
      name: 'Happy path: Edit pins shows while the filter is on and something is pinned',
      overrides: { onlyPinned: true, pinnedCount: 3 },
      shown: true,
    },
  ];
  for (const tc of editPinsCases) {
    it(tc.name, () => {
      mount(tc.overrides);

      expect(screen.queryByRole('button', { name: message('sheet.editPins') }) !== null).toBe(
        tc.shown,
      );
    });
  }

  it('Happy path: Edit pins calls its handler', async () => {
    const user = userEvent.setup();
    const { props: merged } = mount({ onlyPinned: true, pinnedCount: 1 });

    await user.click(screen.getByRole('button', { name: message('sheet.editPins') }));

    expect(merged.onEditPins).toHaveBeenCalledTimes(1);
  });

  it('Edge: the toggle is announced through the sheet announcer, never a second live region of its own', async () => {
    const user = userEvent.setup();
    const store = createLayoutStore(
      { sheetId: 's', sections: arrangeSections(), layout: undefined },
      {
        client: createApiClient({
          baseUrl: 'http://api.test',
          transport: createFakeTransport(successOutcome({})).transport,
        }),
        store: createMemorySheetStore(),
      },
    );
    const onSet = vi.fn();
    render(
      <div className="lg-content">
        <div className="lg-sheet">
          <ArrangeProvider store={store}>
            <SectionIndex {...props({ pinnedCount: 6, onSetOnlyPinned: onSet })} />
          </ArrangeProvider>
        </div>
      </div>,
    );
    const live = screen.getByTestId('arrange-live');
    expect(document.querySelectorAll('[aria-live]')).toHaveLength(1);

    await user.click(screen.getByRole('button', { name: message('sheet.pinnedOnly') }));

    expect(live).toHaveTextContent(messageWith('sheet.pinnedShowing', { count: 6 }));
    expect(document.querySelectorAll('[aria-live]')).toHaveLength(1);
  });
});
