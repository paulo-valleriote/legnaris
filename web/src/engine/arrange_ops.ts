/**
 * The pure staging operations over an {@link Arrangement}: move a Section, pin and
 * unpin a Row, reset to the Template default. Each returns a new arrangement, or
 * the very same reference when the operation changes nothing, so a caller detects
 * a no-op by identity.
 *
 * Rows are never reordered: a pin only lifts a Row to the top of its own Section,
 * and unpinning returns it to its Template position.
 */

import type { SheetSection } from './field_grouping';
import type { Arrangement } from './layout';
import { resolveArrangement } from './layout';

function clamp(index: number, length: number): number {
  return Math.min(Math.max(index, 0), length - 1);
}

/** Moves a Section to [toIndex] (clamped into range). Unknown Section or no change returns the same arrangement. */
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

/**
 * Pins a Row after the ones already pinned, which is where it rises to in its
 * Section. An already pinned Row returns the same arrangement. The key is not
 * validated here; the layout store resolves every staged result against the
 * Template, which drops a key that names no Row.
 */
export function pinRow(arrangement: Arrangement, key: string): Arrangement {
  if (arrangement.pinned.includes(key)) {
    return arrangement;
  }
  return { ...arrangement, pinned: [...arrangement.pinned, key] };
}

/** Unpins a Row, which returns to its Template position. A Row that is not pinned returns the same arrangement. */
export function unpinRow(arrangement: Arrangement, key: string): Arrangement {
  if (!arrangement.pinned.includes(key)) {
    return arrangement;
  }
  return { ...arrangement, pinned: arrangement.pinned.filter((pinned) => pinned !== key) };
}

/** The Template default: Template order, nothing pinned. */
export function resetArrangement(sections: readonly SheetSection[]): Arrangement {
  return resolveArrangement(sections, undefined);
}
