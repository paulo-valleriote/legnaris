import type { Layout } from '../api/models/layout';
import type { SheetRow, SheetSection } from './field_grouping';
import { rowKey } from './field_grouping';

export interface Arrangement {
  readonly sectionOrder: readonly string[];
  readonly pinned: readonly string[];
}

export interface ArrangedRow {
  readonly key: string;
  readonly row: SheetRow;
  readonly sectionKey: string;
  readonly pinned: boolean;
}

export interface ArrangedSection {
  readonly section: SheetSection;
  readonly rows: readonly ArrangedRow[];
}

function uniqueKnown(
  keys: readonly string[],
  known: ReadonlySet<string>,
  isKnown?: (key: string) => boolean,
): string[] {
  const seen = new Set<string>();
  const kept: string[] = [];
  for (const key of keys) {
    if ((known.has(key) || isKnown?.(key) === true) && !seen.has(key)) {
      seen.add(key);
      kept.push(key);
    }
  }
  return kept;
}

function withMissingAppended(
  listed: readonly string[],
  templateOrder: readonly string[],
): readonly string[] {
  const kept = uniqueKnown(listed, new Set(templateOrder));
  const present = new Set(kept);
  return [...kept, ...templateOrder.filter((key) => !present.has(key))];
}

export function resolveArrangement(
  sections: readonly SheetSection[],
  layout: Layout | undefined,
  isRowKey?: (key: string) => boolean,
): Arrangement {
  const everyRow = new Set(sections.flatMap((section) => section.rows.map((row) => rowKey(row))));
  return {
    sectionOrder: withMissingAppended(
      layout?.sectionOrder ?? [],
      sections.map((section) => section.key),
    ),
    pinned: uniqueKnown(layout?.pinned ?? [], everyRow, isRowKey),
  };
}

function sameList(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((key, index) => key === right[index]);
}

export function arrangementsEqual(left: Arrangement, right: Arrangement): boolean {
  return sameList(left.sectionOrder, right.sectionOrder) && sameList(left.pinned, right.pinned);
}

export function arrangementToLayout(
  sections: readonly SheetSection[],
  arrangement: Arrangement,
  isRowKey?: (key: string) => boolean,
): Layout | null {
  const resolved = resolveArrangement(sections, arrangement, isRowKey);
  const template = resolveArrangement(sections, undefined);
  const sectionOrder = sameList(resolved.sectionOrder, template.sectionOrder)
    ? []
    : resolved.sectionOrder;
  if (sectionOrder.length === 0 && resolved.pinned.length === 0) {
    return null;
  }
  return { sectionOrder, pinned: resolved.pinned };
}

export function arrangementView(
  sections: readonly SheetSection[],
  arrangement: Arrangement,
): readonly ArrangedSection[] {
  const resolved = resolveArrangement(sections, arrangement);
  const byKey = new Map(sections.map((section) => [section.key, section]));
  const pinOrder = new Map(resolved.pinned.map((key, index) => [key, index]));

  return resolved.sectionOrder
    .flatMap((sectionKey) => byKey.get(sectionKey) ?? [])
    .map((section) => {
      const rows = section.rows.map((row) => {
        const key = rowKey(row);
        return { key, row, sectionKey: section.key, pinned: pinOrder.has(key) };
      });
      const pinned = rows
        .filter((entry) => entry.pinned)
        .sort((a, b) => (pinOrder.get(a.key) ?? 0) - (pinOrder.get(b.key) ?? 0));
      return { section, rows: [...pinned, ...rows.filter((entry) => !entry.pinned)] };
    });
}

function isConfigProxy(path: string): boolean {
  return path.split('.')[0] === 'config';
}

export function isNumberRow(row: SheetRow): boolean {
  if (row.kind !== 'normal') {
    return false;
  }
  const { path, schema } = row.field;
  return schema.type === 'number' && !schema.flag && !(schema.isEditable && isConfigProxy(path));
}
