import type { SheetSection } from './field_grouping';
import type { Arrangement } from './layout';
import { resolveArrangement } from './layout';

function clamp(index: number, length: number): number {
  return Math.min(Math.max(index, 0), length - 1);
}

export function moveSection(
  arrangement: Arrangement,
  sectionKey: string,
  toIndex: number,
): Arrangement {
  const from = arrangement.sectionOrder.indexOf(sectionKey);
  if (from < 0 || !Number.isFinite(toIndex)) {
    return arrangement;
  }
  const to = clamp(Math.trunc(toIndex), arrangement.sectionOrder.length);
  if (to === from) {
    return arrangement;
  }
  const next = [...arrangement.sectionOrder];
  next.splice(from, 1);
  next.splice(to, 0, sectionKey);
  return { ...arrangement, sectionOrder: next };
}

export function pinRow(arrangement: Arrangement, key: string): Arrangement {
  if (arrangement.pinned.includes(key)) {
    return arrangement;
  }
  return { ...arrangement, pinned: [...arrangement.pinned, key] };
}

export function unpinRow(arrangement: Arrangement, key: string): Arrangement {
  if (!arrangement.pinned.includes(key)) {
    return arrangement;
  }
  return { ...arrangement, pinned: arrangement.pinned.filter((pinned) => pinned !== key) };
}

export function resetArrangement(sections: readonly SheetSection[]): Arrangement {
  return resolveArrangement(sections, undefined);
}
