/**
 * Moves the person's attention to one Row or Section of the open sheet.
 *
 * It is the one place that knows how to bring either into view. The caller may hand in
 * the view controls: they turn the pinned filter off and expand a collapsed Section
 * first, synchronously, so the target is drawn and visible before it is scrolled to.
 * A Row is then scrolled clear of the sticky footer and the strip, marked as revealed
 * and focused; a Section is scrolled under the strip and its title control is focused.
 * Callers only name the target by its `data-row-key` or `data-section-key`.
 */

import { sectionKeyOfRow } from '../engine/field_grouping';

/** The view changes a reveal may need; each must have rendered by the time it returns. */
export interface RevealView {
  /** Turns the pinned filter off and expands the Section named [sectionKey]. */
  reveal(sectionKey: string): void;
  /** Turns the pinned filter off so every Row is drawn. */
  showAll(): void;
}

/** The attribute that paints the revealed Row; the stylesheet owns its look. */
export const REVEALED_ATTRIBUTE = 'data-revealed';

/** The attribute the preview mark carries so a Row can be found by what it shows. */
export const MARK_ATTRIBUTE = 'data-mark';

function rowElements(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>('[data-row-key]'));
}

/** True when the person asks the system for less motion; scrolling is then instant. */
function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/**
 * The key of the first Row, in the order drawn, whose preview mark is
 * "recalculated"; `undefined` when no drawn Row carries one.
 */
export function firstRecalculatedRowKey(root: ParentNode = document): string | undefined {
  for (const row of rowElements(root)) {
    if (row.querySelector(`[${MARK_ATTRIBUTE}="recalculated"]`) !== null) {
      return row.dataset.rowKey;
    }
  }
  return undefined;
}

/**
 * Reveals the first Row, in the order drawn, whose preview mark is "recalculated". Under the
 * pinned filter such a Row may not be drawn, so [view] first shows every Row.
 *
 * Returns `false` when no Row carries that mark.
 */
export function revealFirstRecalculated(root: ParentNode = document, view?: RevealView): boolean {
  view?.showAll();
  const key = firstRecalculatedRowKey(root);
  return key !== undefined && revealRow(key, root, view);
}

/**
 * Scrolls the Row named [rowKey] into view, highlights it and focuses it.
 *
 * Returns `false` and changes nothing when no drawn Row has that key, so a
 * stale key never throws or moves the page. [view], when given, first makes
 * the Row's Section visible.
 */
export function revealRow(rowKey: string, root: ParentNode = document, view?: RevealView): boolean {
  view?.reveal(sectionKeyOfRow(rowKey));
  const row = rowElements(root).find((candidate) => candidate.dataset.rowKey === rowKey);
  if (row === undefined) {
    return false;
  }

  for (const previous of root.querySelectorAll<HTMLElement>(`[${REVEALED_ATTRIBUTE}]`)) {
    previous.removeAttribute(REVEALED_ATTRIBUTE);
  }

  const hadTabIndex = row.hasAttribute('tabindex');
  if (!hadTabIndex) {
    row.setAttribute('tabindex', '-1');
  }
  row.setAttribute(REVEALED_ATTRIBUTE, '');
  row.addEventListener(
    'blur',
    () => {
      row.removeAttribute(REVEALED_ATTRIBUTE);
      if (!hadTabIndex) {
        row.removeAttribute('tabindex');
      }
    },
    { once: true },
  );

  row.scrollIntoView({
    block: 'center',
    behavior: prefersReducedMotion() ? 'auto' : 'smooth',
  });
  row.focus({ preventScroll: true });
  return true;
}

/** The control that opens and closes a Section, or the Arrange handle while arranging. */
const SECTION_CONTROL_SELECTOR = '.lg-section__toggle, .lg-arrange-handle';

/**
 * Scrolls the Section named [sectionKey] under the strip and focuses its title
 * control, opening the Section first through [view] when it is collapsed or filtered out.
 *
 * Scrolling is left to the stylesheet's `scroll-behavior`, so reduced motion is honoured
 * there. Returns `false` and changes nothing when no drawn Section has that key.
 */
export function revealSection(
  sectionKey: string,
  root: ParentNode = document,
  view?: RevealView,
): boolean {
  view?.reveal(sectionKey);
  const section = Array.from(root.querySelectorAll<HTMLElement>('[data-section-key]')).find(
    (candidate) => candidate.dataset.sectionKey === sectionKey,
  );
  if (section === undefined) {
    return false;
  }
  section.scrollIntoView({ block: 'start' });
  section.querySelector<HTMLElement>(SECTION_CONTROL_SELECTOR)?.focus({ preventScroll: true });
  return true;
}

/** The region that scrolled the sheet before the document did; the window scrolls unless it is a scroll container. */
const SCROLL_REGION_SELECTOR = '.lg-content';

/**
 * Scrolls the sheet back to its heading. It moves the scrolling region (or the window when there is
 * none) to its start and leaves focus where it is, so a keyboard user stays on the control they used.
 */
export function revealTop(root: ParentNode = document): void {
  const behavior = prefersReducedMotion() ? 'auto' : 'smooth';
  const region = root.querySelector<HTMLElement>(SCROLL_REGION_SELECTOR);
  const regionScrolls =
    region !== null && /^(auto|scroll)$/.test(getComputedStyle(region).overflowY);
  if (region !== null && regionScrolls && typeof region.scrollTo === 'function') {
    region.scrollTo({ top: 0, behavior });
    return;
  }
  if (typeof window.scrollTo === 'function') {
    window.scrollTo({ top: 0, behavior });
  }
}
