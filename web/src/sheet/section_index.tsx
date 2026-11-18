import { useCallback, useEffect, useRef, useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { PinStar } from '../engine/pin_star';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { useAnnounce } from './arrange_frames';
import { useActiveSection } from './use_active_section';
import './section_index.css';

export interface SectionLink {
  readonly key: string;
  readonly label: string;
  readonly changed?: number;
}

export interface SectionIndexProps {
  readonly sheetName: string;
  readonly onTop: () => void;
  readonly sections: readonly SectionLink[];
  readonly onlyPinned: boolean;
  readonly filterDisabled: boolean;
  readonly pinnedCount: number;
  readonly onSetOnlyPinned: (onlyPinned: boolean) => void;
  readonly onJump: (key: string) => void;
  readonly onEditPins: () => void;
  readonly trailing?: ReactNode;
  readonly epoch: string;
}

export function centerInline(nav: HTMLElement, chip: HTMLElement): void {
  const navBox = nav.getBoundingClientRect();
  const chipBox = chip.getBoundingClientRect();
  const offset = chipBox.left + chipBox.width / 2 - (navBox.left + navBox.width / 2);
  if (offset !== 0) {
    nav.scrollLeft += offset;
  }
}

export interface HiddenEdges {
  readonly start: boolean;
  readonly end: boolean;
}

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
          <Button
            variant="outline"
            size="sm"
            className="lg-sectionbar__action"
            label={message('sheet.editPins')}
            onClick={onEditPins}
          />
        )}
      </div>
    </div>
  );
}
