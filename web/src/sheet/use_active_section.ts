import { useCallback, useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';

const SCROLLER_SELECTOR = '.lg-content';

const SCROLLING_OVERFLOW = /^(auto|scroll|overlay)$/;

const SHEET_SELECTOR = '.lg-sheet';

const END_TOLERANCE = 2;

const JUMP_SETTLE_MS = 900;

const REACHED_LINE_OFFSET = 16;

const PIXELS = 'px';

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

export interface ActiveSection {
  readonly activeKey: string | undefined;
  readonly pin: (key: string) => void;
}

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
