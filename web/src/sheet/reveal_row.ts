import { sectionKeyOfRow } from '../engine/field_grouping';

export interface RevealView {
  reveal(sectionKey: string): void;
  showAll(): void;
}

export const REVEALED_ATTRIBUTE = 'data-revealed';

export const MARK_ATTRIBUTE = 'data-mark';

function rowElements(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>('[data-row-key]'));
}

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export function firstRecalculatedRowKey(root: ParentNode = document): string | undefined {
  for (const row of rowElements(root)) {
    if (row.querySelector(`[${MARK_ATTRIBUTE}="recalculated"]`) !== null) {
      return row.dataset.rowKey;
    }
  }
  return undefined;
}

export function revealFirstRecalculated(root: ParentNode = document, view?: RevealView): boolean {
  view?.showAll();
  const key = firstRecalculatedRowKey(root);
  return key !== undefined && revealRow(key, root, view);
}

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

const SECTION_CONTROL_SELECTOR = '.lg-section__toggle, .lg-arrange-handle';

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

const SCROLL_REGION_SELECTOR = '.lg-content';

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
