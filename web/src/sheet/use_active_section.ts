/**
 * Scrollspy for the section strip: which Section the person is reading.
 *
 * It observes the drawn Sections with an `IntersectionObserver` whose top root margin is the
 * strip's own height (the root is the viewport, because the document scrolls), so a Section is a candidate once it is below the strip. Of the candidates
 * the last one whose top has reached the strip wins, or the first when none has. At the end of
 * the scroll the last Section wins, because a short last Section can never reach the top. A jump
 * pins the chosen Section until the smooth scroll settles. Without an `IntersectionObserver`
 * there is no active Section and nothing throws.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';

/** The region that used to scroll the sheet; it counts as the scroller only while it still does. */
const SCROLLER_SELECTOR = '.lg-content';

const SCROLLING_OVERFLOW = /^(auto|scroll|overlay)$/;

/** The ancestor that holds the drawn Sections. */
const SHEET_SELECTOR = '.lg-sheet';

/** How far from the scroll end still counts as the end, in device pixels. */
const END_TOLERANCE = 2;

/** The longest a jump keeps its Section pinned when the browser fires no `scrollend`. */
const JUMP_SETTLE_MS = 900;

/**
 * How far below the strip a Section's top may sit and still count as reached: a Section a jump
 * brings into view lands one spacing step under the strip, so it must count as the current one.
 */
const REACHED_LINE_OFFSET = 16;

/** The unit of the root margin's top edge; the band's other edges are zero or a percentage. */
const PIXELS = 'px';

/** The share of the viewport, below the strip, in which a Section counts as being read. */
const BAND_FRACTION = 0.4;

function sectionElements(sheet: Element, keys: readonly string[]): Map<string, Element> {
  const found = new Map<string, Element>();
  for (const element of sheet.querySelectorAll('[data-section-key]')) {
    const key = element.getAttribute('data-section-key');
    if (key !== null && keys.includes(key)) {
      found.set(key, element);
    }
  }
  return found;
}

/**
 * The element that scrolls the strip's Sections: the content region only while its own overflow
 * makes it a scroll container, otherwise `null`, which stands for the viewport (the document
 * scrolls). Observing a region that does not scroll would leave the scrollspy blind.
 */
export function findScroller(strip: Element): Element | null {
  const region = strip.closest(SCROLLER_SELECTOR);
  if (region === null) {
    return null;
  }
  return SCROLLING_OVERFLOW.test(getComputedStyle(region).overflowY) ? region : null;
}

function atScrollEnd(scroller: Element | null): boolean {
  const target = scroller ?? document.documentElement;
  const scrolls = target.scrollHeight > target.clientHeight;
  return scrolls && target.scrollTop + target.clientHeight >= target.scrollHeight - END_TOLERANCE;
}

/** What the strip reads: the active Section key (if known) and a way to pin one after a jump. */
export interface ActiveSection {
  readonly activeKey: string | undefined;
  /** Marks [key] active at once and ignores the scroll position until the scroll settles. */
  readonly pin: (key: string) => void;
}

/**
 * Tracks the Section being read among [keys] (their order is the reading order), measured
 * against the strip in [stripRef]. [epoch] changes whenever the drawn Section elements are
 * replaced (a different frame, for instance), so the observer is rebuilt on the new ones.
 */
export function useActiveSection(
  keys: readonly string[],
  stripRef: RefObject<HTMLElement | null>,
  epoch: string,
): ActiveSection {
  const [activeKey, setActiveKey] = useState<string | undefined>(undefined);
  const pinTimer = useRef<number | undefined>(undefined);
  const recompute = useRef<() => void>(() => undefined);
  const keysId = keys.join('\u0000');

  useEffect(() => {
    const strip = stripRef.current;
    const currentKeys = keysId === '' ? [] : keysId.split('\u0000');
    if (strip === null || currentKeys.length === 0 || typeof IntersectionObserver === 'undefined') {
      setActiveKey(undefined);
      recompute.current = () => undefined;
      return;
    }
    const sheet = strip.closest(SHEET_SELECTOR);
    const scroller = findScroller(strip);
    if (sheet === null) {
      setActiveKey(undefined);
      return;
    }
    const elements = sectionElements(sheet, currentKeys);
    const inBand = new Set<string>();

    const resolve = (): void => {
      if (pinTimer.current !== undefined) {
        return;
      }
      if (atScrollEnd(scroller)) {
        setActiveKey(currentKeys[currentKeys.length - 1]);
        return;
      }
      const candidates = currentKeys.filter((key) => inBand.has(key));
      const line = strip.getBoundingClientRect().bottom + REACHED_LINE_OFFSET;
      const reached = candidates.filter(
        (key) => (elements.get(key)?.getBoundingClientRect().top ?? line) <= line,
      );
      const chosen = reached[reached.length - 1] ?? candidates[0];
      setActiveKey((previous) => chosen ?? previous ?? currentKeys[0]);
    };
    recompute.current = resolve;

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const key = entry.target.getAttribute('data-section-key');
          if (key === null) {
            continue;
          }
          if (entry.isIntersecting) {
            inBand.add(key);
          } else {
            inBand.delete(key);
          }
        }
        resolve();
      },
      {
        root: scroller,
        rootMargin: `-${strip.offsetHeight}${PIXELS} 0% -${Math.round(BAND_FRACTION * 100)}% 0%`,
      },
    );
    for (const element of elements.values()) {
      observer.observe(element);
    }
    const onScroll = (): void => resolve();
    const scrollTarget: EventTarget = scroller ?? window;
    scrollTarget.addEventListener('scroll', onScroll, { passive: true });
    resolve();

    return () => {
      observer.disconnect();
      scrollTarget.removeEventListener('scroll', onScroll);
      recompute.current = () => undefined;
    };
  }, [keysId, stripRef, epoch]);

  useEffect(
    () => () => {
      window.clearTimeout(pinTimer.current);
    },
    [],
  );

  const pin = useCallback((key: string) => {
    window.clearTimeout(pinTimer.current);
    const release = (): void => {
      window.clearTimeout(pinTimer.current);
      pinTimer.current = undefined;
      recompute.current();
    };
    pinTimer.current = window.setTimeout(release, JUMP_SETTLE_MS);
    setActiveKey(key);
    document.addEventListener('scrollend', release, { once: true, capture: true });
  }, []);

  return { activeKey, pin };
}
