export interface PresetSectionTarget {
  readonly key: string;
  readonly label: string;
}

export interface SectionPosition {
  readonly key: string;
  readonly top: number;
}

export const PRESET_SELECTOR = '.lg-preset';

export const PRESET_SECTION_SELECTOR = '[data-preset-section]';

export const NAV_SECTION_SELECTOR = 'h2';

const END_TOLERANCE = 2;

function collapse(text: string): string {
  return text.replace(/\s+/g, ' ').trim();
}

function headingLabel(heading: HTMLElement): string {
  const title = heading.querySelector<HTMLElement>(':scope > span:first-child');
  return collapse((title ?? heading).textContent ?? '');
}

export function presentSections(root: ParentNode, skipKey: string): readonly PresetSectionTarget[] {
  const targets: PresetSectionTarget[] = [];
  for (const wrapper of root.querySelectorAll<HTMLElement>(PRESET_SECTION_SELECTOR)) {
    const key = wrapper.dataset.presetSection;
    if (key === undefined || key === skipKey || wrapper.childElementCount === 0) {
      continue;
    }
    const heading = wrapper.querySelector<HTMLElement>(NAV_SECTION_SELECTOR);
    if (heading === null) {
      continue;
    }
    targets.push({ key, label: headingLabel(heading) });
  }
  return targets;
}

export function sectionHeading(root: ParentNode, key: string): HTMLElement | undefined {
  for (const wrapper of root.querySelectorAll<HTMLElement>(PRESET_SECTION_SELECTOR)) {
    if (wrapper.dataset.presetSection !== key) {
      continue;
    }
    return wrapper.querySelector<HTMLElement>(NAV_SECTION_SELECTOR) ?? undefined;
  }
  return undefined;
}

export function prefersReducedMotion(): boolean {
  return (
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

export function revealHeading(heading: HTMLElement, reduced: boolean): void {
  heading.scrollIntoView({ block: 'start', behavior: reduced ? 'auto' : 'smooth' });
  if (!heading.hasAttribute('tabindex')) {
    heading.setAttribute('tabindex', '-1');
    heading.addEventListener(
      'blur',
      () => {
        heading.removeAttribute('tabindex');
      },
      { once: true },
    );
  }
  heading.focus({ preventScroll: true });
}

export function currentSectionKey(
  positions: readonly SectionPosition[],
  line: number,
  atEnd: boolean,
): string | undefined {
  if (positions.length === 0) {
    return undefined;
  }
  if (atEnd) {
    return positions[positions.length - 1].key;
  }
  let current = positions[0].key;
  for (const position of positions) {
    if (position.top <= line) {
      current = position.key;
    }
  }
  return current;
}

export function atScrollEnd(target: {
  readonly scrollTop: number;
  readonly clientHeight: number;
  readonly scrollHeight: number;
}): boolean {
  return (
    target.scrollHeight > target.clientHeight &&
    target.scrollTop + target.clientHeight >= target.scrollHeight - END_TOLERANCE
  );
}
