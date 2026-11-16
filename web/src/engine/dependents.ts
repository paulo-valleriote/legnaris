import type { SheetField, SheetSection } from './field_grouping';
import { isNumberRow } from './layout';

export const MAX_BESIDE_DEPENDENTS = 2;

export type DependentsIndex = ReadonlyMap<string, readonly SheetField[]>;

function dependencyPath(segments: readonly string[]): string | undefined {
  if (segments.length === 0 || segments.some((segment) => segment === '')) {
    return undefined;
  }
  return segments.join('.');
}

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

export function dependentsAreStale(sourceEdited: boolean, previewed: boolean): boolean {
  return sourceEdited && !previewed;
}
