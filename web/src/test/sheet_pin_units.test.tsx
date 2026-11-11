import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { arrangementView, resolveArrangement } from '../engine/layout';
import { message, messageWith } from '../l10n/strings';
import { markedRowCounts } from '../sheet/preview_summary';
import { revealTop } from '../sheet/reveal_row';
import { SectionIndex } from '../sheet/section_index';
import { arrangeSections } from './layout_fixtures';

const sections = arrangeSections();

describe('markedRowCounts', () => {
  const view = arrangementView(sections, resolveArrangement(sections, undefined));

  it('Edge: no marks, no counts', () => {
    expect(markedRowCounts(view, undefined).size).toBe(0);
  });

  it('Edge: only Sections with a marked Row appear, once per Row', () => {
    const counts = markedRowCounts(view, {
      modified: new Set(['alpha.one']),
      recalculated: new Set(['alpha.one', 'alpha.two', 'beta.stats.str']),
    } as never);

    expect([...counts]).toEqual([
      ['alpha', 2],
      ['beta', 1],
    ]);
  });

  it('Edge: a path naming no drawn Row counts nowhere', () => {
    const counts = markedRowCounts(view, {
      modified: new Set(['ghost.path']),
      recalculated: new Set(),
    } as never);

    expect(counts.size).toBe(0);
  });
});

describe('revealTop', () => {
  afterEach(() => {
    Reflect.deleteProperty(Element.prototype, 'scrollTo');
    vi.restoreAllMocks();
  });

  it('Edge: with no scrolling region it scrolls the window and never throws', () => {
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined);
    render(<div />);

    expect(() => revealTop(document)).not.toThrow();
    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: 0 }));
  });

  it('Happy path: the scrolling region goes to its start', () => {
    const scrollTo = vi.fn();
    Element.prototype.scrollTo = scrollTo as unknown as typeof Element.prototype.scrollTo;
    render(<div className="lg-content" style={{ overflowY: 'auto' }} />);

    revealTop(document);

    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: 0 }));
  });
});

describe('the section strip leading slot and count dots', () => {
  const base = {
    sheetName: 'Aragorn',
    onTop: vi.fn(),
    onlyPinned: false,
    filterDisabled: false,
    pinnedCount: 0,
    onSetOnlyPinned: vi.fn(),
    onJump: vi.fn(),
    onEditPins: vi.fn(),
    epoch: 'play',
  };
  const links = [
    { key: 'a', label: 'Alpha', changed: 3 },
    { key: 'b', label: 'Beta', changed: 0 },
    { key: 'c', label: 'Gamma' },
  ];

  it('Edge: an empty sheet name draws no chip', () => {
    render(<SectionIndex {...base} sheetName="" sections={links} />);

    expect(screen.queryByRole('button', { name: /back to top/ })).not.toBeInTheDocument();
  });

  it('Happy path: the chip calls onTop and the filter keeps its name', async () => {
    const onTop = vi.fn();
    render(<SectionIndex {...base} onTop={onTop} sections={links} />);

    await userEvent.setup().click(screen.getByRole('button', { name: 'Aragorn, back to top' }));

    expect(onTop).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: message('sheet.pinnedOnly') })).toBeInTheDocument();
  });

  it('Edge: only a link with a positive count carries a dot, and the count is in its name', () => {
    render(<SectionIndex {...base} sections={links} />);
    const nav = screen.getByRole('navigation', { name: message('sheet.sections') });

    expect(
      within(nav).getByRole('button', {
        name: messageWith('sheet.sectionLinkChanged', { label: 'Alpha', count: 3 }),
      }),
    ).toBeInTheDocument();
    expect(nav.querySelectorAll('.lg-sectionbar__dot')).toHaveLength(1);
    expect(
      within(nav).getByRole('button', {
        name: messageWith('sheet.sectionLink', { label: 'Beta' }),
      }),
    ).toBeInTheDocument();
    expect(
      within(nav).getByRole('button', {
        name: messageWith('sheet.sectionLink', { label: 'Gamma' }),
      }),
    ).toBeInTheDocument();
  });
});
