export interface PresetSectionTarget {
  readonly key: string;
  readonly label: string;
  readonly sectionKey: string;
}

export interface PresetSectionGroup {
  readonly id: string;
  readonly label: string;
  readonly sections: readonly string[];
}

export interface SectionPosition {
  readonly key: string;
  readonly top: number;
}

export const PRESET_SELECTOR = '.lg-preset';

export const PRESET_SECTION_SELECTOR = '[data-preset-section]';

export const NAV_SECTION_SELECTOR = 'h2, h3';

const END_TOLERANCE = 2;

function collapse(text: string): string {
  return text.replace(/\s+/g, ' ').trim();
}

function headingLabel(heading: HTMLElement): string {
  const title = heading.querySelector<HTMLElement>(':scope > span:first-child');
  return collapse((title ?? heading).textContent ?? '');
}

function presentWrappers(
  root: ParentNode,
  skipKey: string | undefined,
): ReadonlyMap<string, HTMLElement> {
  const present = new Map<string, HTMLElement>();
  for (const wrapper of root.querySelectorAll<HTMLElement>(PRESET_SECTION_SELECTOR)) {
    const key = wrapper.dataset.presetSection;
    if (key === undefined || key === skipKey || wrapper.childElementCount === 0) {
      continue;
    }
    if (!present.has(key)) {
      present.set(key, wrapper);
    }
  }
  return present;
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
    targets.push({ key, label: headingLabel(heading), sectionKey: key });
  }
  return targets;
}

export function presentTargets(
  root: ParentNode,
  groups: readonly PresetSectionGroup[],
  skipKey: string | undefined,
): readonly PresetSectionTarget[] {
  const present = presentWrappers(root, skipKey);
  if (present.size === 0) {
    return [];
  }
  const targets: PresetSectionTarget[] = [];
  for (const group of groups) {
    const sectionKey = group.sections.find((key) => key !== skipKey && present.has(key));
    if (sectionKey === undefined) {
      continue;
    }
    targets.push({ key: group.id, label: group.label, sectionKey });
  }
  return targets;
}

export function sectionAnchor(root: ParentNode, key: string): HTMLElement | undefined {
  for (const wrapper of root.querySelectorAll<HTMLElement>(PRESET_SECTION_SELECTOR)) {
    if (wrapper.dataset.presetSection !== key || wrapper.childElementCount === 0) {
      continue;
    }
    return wrapper.querySelector<HTMLElement>(NAV_SECTION_SELECTOR) ?? wrapper;
  }
  return undefined;
}

export function prefersReducedMotion(): boolean {
  return (
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

export function revealTarget(target: HTMLElement, reduced: boolean): void {
  target.scrollIntoView({ block: 'start', behavior: reduced ? 'auto' : 'smooth' });
  if (!target.hasAttribute('tabindex')) {
    target.setAttribute('tabindex', '-1');
    target.addEventListener(
      'blur',
      () => {
        target.removeAttribute('tabindex');
      },
      { once: true },
    );
  }
  target.focus({ preventScroll: true });
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
