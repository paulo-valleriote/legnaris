/**
 * Derived values beside the input that drives them (sheet-object brief, A12). Everything here is
 * pure and reads only the decoded schema: the inverse of `dependsOn`, direct edges only. Nothing
 * evaluates a formula; a value beside its input is the one the server last computed, and
 * {@link dependentsAreStale} says whether the player has edited the input since.
 */

import type { SheetField, SheetSection } from './field_grouping';
import { isNumberRow } from './layout';

/** The most direct dependents an input shows beside itself; an input that feeds more shows none. */
export const MAX_BESIDE_DEPENDENTS = 2;

/** Each leaf path mapped to the derived leaves that list it directly in their `dependsOn`, in tree order. */
export type DependentsIndex = ReadonlyMap<string, readonly SheetField[]>;

function dependencyPath(segments: readonly string[]): string | undefined {
  if (segments.length === 0 || segments.some((segment) => segment === '')) {
    return undefined;
  }
  return segments.join('.');
}

/**
 * Inverts `dependsOn` over every leaf of [sections]. Only derived (non-editable) leaves are
 * dependents. An edge to a path that is no leaf of the tree (an external or stale reference), a
 * malformed or empty dependency, a repeated dependency and a leaf that lists itself are dropped, so
 * a cycle can never put a leaf beside itself. Edges are direct only: a leaf fed by a derived leaf is
 * not a dependent of that leaf's own inputs.
 */
export function buildDependentsIndex(sections: readonly SheetSection[]): DependentsIndex {
  const leaves = sections.flatMap((section) => section.leaves);
  const known = new Set(leaves.map((leaf) => leaf.path));
  const index = new Map<string, SheetField[]>();
  const placed = new Set<string>();

  for (const leaf of leaves) {
    if (leaf.schema.isEditable || placed.has(leaf.path)) {
      continue;
    }
    placed.add(leaf.path);
    const seen = new Set<string>();
    for (const segments of leaf.schema.dependsOn ?? []) {
      const path = dependencyPath(segments);
      if (path === undefined || path === leaf.path || seen.has(path) || !known.has(path)) {
        continue;
      }
      seen.add(path);
      const list = index.get(path);
      if (list === undefined) {
        index.set(path, [leaf]);
      } else {
        list.push(leaf);
      }
    }
  }
  return index;
}

/**
 * The derived leaves to draw beside [source]. Only an editable number input that draws as a number
 * (not a `config.*` switch) has any, and only when it feeds one or two derived leaves directly:
 * zero, or more than {@link MAX_BESIDE_DEPENDENTS}, shows nothing, because a long list is not a
 * readout. The count is of the schema, so it never changes with pins.
 */
export function dependentsOf(index: DependentsIndex, source: SheetField): readonly SheetField[] {
  if (!source.schema.isEditable || !isNumberRow({ kind: 'normal', field: source })) {
    return [];
  }
  const direct = index.get(source.path) ?? [];
  if (direct.length === 0 || direct.length > MAX_BESIDE_DEPENDENTS) {
    return [];
  }
  return direct;
}

/**
 * Whether the dependents beside an input no longer match it: the input has been edited and no
 * Preview has confirmed that edit yet. A Preview or a Save settles it; any later edit unsettles it
 * again, because the edit store clears the preview marks on every change. It errs toward stale:
 * after an edit elsewhere every edited input's dependents read stale until the next Preview.
 */
export function dependentsAreStale(sourceEdited: boolean, previewed: boolean): boolean {
  return sourceEdited && !previewed;
}
