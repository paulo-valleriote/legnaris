/**
 * The section strip: the one sticky row at the top of the sheet's scroll region that
 * holds the pinned filter, the sheet's name and the links to the Sections.
 *
 * Its leading slot keeps the sheet's identity in view once the heading has scrolled away: a star
 * toggle for the pinned filter (its name, "Pinned only", is drawn from 40rem up and read to
 * assistive technology at every width) and a compact chip with the sheet's name that scrolls back
 * to the top. A link whose Section holds Rows marked by the last Preview carries a count dot.
 *
 * The links are plain buttons in a `nav` with a roving tab index: the whole row is one
 * tab stop, the arrow keys, Home and End move between links, and Enter or Space jumps.
 * It is not a tab list, because the links move the reader and never swap a panel. The
 * link of the Section being read carries `aria-current="location"`. With one Section
 * there are no links; with none the strip is not drawn at all.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { PinStar } from '../engine/pin_star';
import { message, messageWith } from '../l10n/strings';
import { useAnnounce } from './arrange_frames';
import { useActiveSection } from './use_active_section';
import './section_index.css';

/** One link of the strip: a Section in the order it is drawn. */
export interface SectionLink {
  readonly key: string;
  readonly label: string;
  /** How many of the Section's drawn Rows carry a preview mark; absent or zero draws no dot. */
  readonly changed?: number;
}

/** The props of {@link SectionIndex}. */
export interface SectionIndexProps {
  /** The sheet's name, shown as the chip that scrolls back to the top; empty draws no chip. */
  readonly sheetName: string;
  /** Scrolls the sheet back to its heading. */
  readonly onTop: () => void;
  /** The Sections the strip links to, in drawn order; empty draws no links. */
  readonly sections: readonly SectionLink[];
  /** True while only pinned Rows are shown. */
  readonly onlyPinned: boolean;
  /** True while the filter cannot be used (Arrange mode); it then reads as off. */
  readonly filterDisabled: boolean;
  /** How many Rows are pinned, announced when the filter turns on. */
  readonly pinnedCount: number;
  /** Turns the filter on or off. */
  readonly onSetOnlyPinned: (onlyPinned: boolean) => void;
  /** Goes to a Section: scrolls it under the strip, opens it and focuses its title. */
  readonly onJump: (key: string) => void;
  /** Enters Arrange mode to edit pins; offered while the filter is on and something is pinned. */
  readonly onEditPins: () => void;
  /** An extra control drawn at the trailing edge of the strip, such as collapsing every Section. */
  readonly trailing?: ReactNode;
  /** Changes whenever the drawn Section elements are replaced, so the scrollspy rebuilds. */
  readonly epoch: string;
}

/**
 * Scrolls only the strip's own row, never an ancestor, so the active link comes to the centre of
 * the row without cancelling the smooth page scroll a jump has just started. A link already
 * centred moves nothing.
 */
export function centerInline(nav: HTMLElement, chip: HTMLElement): void {
  const navBox = nav.getBoundingClientRect();
  const chipBox = chip.getBoundingClientRect();
  const offset = chipBox.left + chipBox.width / 2 - (navBox.left + navBox.width / 2);
  if (offset !== 0) {
    nav.scrollLeft += offset;
  }
}

/** Which edges of the strip's row hide links: a fade at an edge says there is more past it. */
export interface HiddenEdges {
  readonly start: boolean;
  readonly end: boolean;
}

/** Reads the hidden edges of a horizontally scrolling row; a row that fits hides none. */
export function hiddenEdges(
  nav: Pick<HTMLElement, 'scrollLeft' | 'scrollWidth' | 'clientWidth'>,
): HiddenEdges {
  const rest = nav.scrollWidth - nav.clientWidth;
  if (rest <= EDGE_TOLERANCE) {
    return { start: false, end: false };
  }
  return {
    start: nav.scrollLeft > EDGE_TOLERANCE,
    end: nav.scrollLeft < rest - EDGE_TOLERANCE,
  };
}

/** How far from an edge still counts as at it, in device pixels. */
const EDGE_TOLERANCE = 2;

function nextIndex(key: string, index: number, count: number): number | undefined {
  switch (key) {
    case 'ArrowRight':
      return (index + 1) % count;
    case 'ArrowLeft':
      return (index - 1 + count) % count;
    case 'Home':
      return 0;
    case 'End':
      return count - 1;
    default:
      return undefined;
  }
}

/** The sticky strip above the sheet body. */
export function SectionIndex({
  sheetName,
  onTop,
  sections,
  onlyPinned,
  filterDisabled,
  pinnedCount,
  onSetOnlyPinned,
  onJump,
  onEditPins,
  trailing,
  epoch,
}: SectionIndexProps) {
  const stripRef = useRef<HTMLDivElement>(null);
  const navRef = useRef<HTMLElement>(null);
  const chipRefs = useRef(new Map<string, HTMLButtonElement>());
  const [rovingKey, setRovingKey] = useState<string | null>(null);
  const [edges, setEdges] = useState<HiddenEdges>({ start: false, end: false });
  const announce = useAnnounce();
  const keys = sections.map((section) => section.key);
  const { activeKey, pin } = useActiveSection(keys, stripRef, epoch);
  const pressed = onlyPinned && !filterDisabled;
  const tabStopKey =
    rovingKey !== null && keys.includes(rovingKey) ? rovingKey : (activeKey ?? keys[0]);

  useEffect(() => {
    if (activeKey === undefined) {
      return;
    }
    const chip = chipRefs.current.get(activeKey);
    if (navRef.current !== null && chip !== undefined) {
      centerInline(navRef.current, chip);
    }
  }, [activeKey]);

  const sectionCount = sections.length;
  useEffect(() => {
    const nav = navRef.current;
    if (nav === null) {
      return;
    }
    const update = (): void => {
      setEdges((previous) => {
        const next = hiddenEdges(nav);
        return previous.start === next.start && previous.end === next.end ? previous : next;
      });
    };
    update();
    nav.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update);
    return () => {
      nav.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, [sectionCount]);

  const jump = useCallback(
    (key: string) => {
      pin(key);
      onJump(key);
    },
    [onJump, pin],
  );

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number): void => {
    const target = nextIndex(event.key, index, sections.length);
    if (target === undefined) {
      return;
    }
    event.preventDefault();
    const key = sections[target].key;
    setRovingKey(key);
    chipRefs.current.get(key)?.focus();
  };

  const toggleFilter = (): void => {
    const next = !onlyPinned;
    onSetOnlyPinned(next);
    announce(
      next
        ? messageWith('sheet.pinnedShowing', { count: pinnedCount })
        : message('sheet.pinnedShowingAll'),
    );
  };

  return (
    <div className="lg-sectionbar" ref={stripRef} data-testid="section-strip">
      <div className="lg-sectionbar__inner">
        <button
          type="button"
          className="lg-sectionbar__filter"
          aria-pressed={pressed}
          disabled={filterDisabled}
          onClick={toggleFilter}
        >
          <PinStar className="lg-sectionbar__star" />
          <span className="lg-sectionbar__filter-text">{message('sheet.pinnedOnly')}</span>
        </button>
        {sheetName !== '' && (
          <button
            type="button"
            className="lg-sectionbar__name"
            aria-label={messageWith('sheet.backToTop', { name: sheetName })}
            onClick={onTop}
          >
            <span className="lg-sectionbar__name-text">{sheetName}</span>
          </button>
        )}
        {sections.length > 1 && (
          <nav
            ref={navRef}
            className="lg-sectionbar__nav"
            data-edge-start={edges.start ? '' : undefined}
            data-edge-end={edges.end ? '' : undefined}
            aria-label={message('sheet.sections')}
            onBlur={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget)) {
                setRovingKey(null);
              }
            }}
          >
            <ul className="lg-sectionbar__list">
              {sections.map((section, index) => (
                <li key={section.key}>
                  <button
                    type="button"
                    className="lg-sectionbar__chip"
                    ref={(element) => {
                      if (element === null) {
                        chipRefs.current.delete(section.key);
                      } else {
                        chipRefs.current.set(section.key, element);
                      }
                    }}
                    tabIndex={section.key === tabStopKey ? 0 : -1}
                    aria-current={section.key === activeKey ? 'location' : undefined}
                    aria-label={
                      (section.changed ?? 0) > 0
                        ? messageWith('sheet.sectionLinkChanged', {
                            label: section.label,
                            count: section.changed ?? 0,
                          })
                        : messageWith('sheet.sectionLink', { label: section.label })
                    }
                    onFocus={() => setRovingKey(section.key)}
                    onKeyDown={(event) => handleKeyDown(event, index)}
                    onClick={() => jump(section.key)}
                  >
                    <span className="lg-sectionbar__chip-label">{section.label}</span>
                    {(section.changed ?? 0) > 0 && (
                      <span className="lg-sectionbar__dot" aria-hidden="true">
                        {section.changed}
                      </span>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          </nav>
        )}
        {trailing !== undefined && <div className="lg-sectionbar__trailing">{trailing}</div>}
        {pressed && pinnedCount > 0 && (
          <button
            type="button"
            className="lg-button lg-button--outline lg-button--sm lg-sectionbar__action"
            onClick={onEditPins}
          >
            {message('sheet.editPins')}
          </button>
        )}
      </div>
    </div>
  );
}
