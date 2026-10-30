/**
 * Layout resolution for the schema-driven sheet renderer (ADR 0001): turns the
 * player's possibly partial, stale or duplicated Layout into one complete
 * {@link Arrangement} over the Template's Sections, and back.
 *
 * Everything here is pure and stateless. The Template supplies the default order;
 * a Layout only deviates from it, so a Section the Layout omits falls back to
 * Template order, a key the Template no longer has is ignored, and a repeated key
 * keeps its first occurrence. Rows are never reordered, only pinned: a pinned Row
 * rises to the top of its own Section, in pin order.
 */

import type { Layout } from '../api/models/layout';
import type { SheetRow, SheetSection } from './field_grouping';
import { rowKey } from './field_grouping';

/**
 * A Layout resolved against a Template: every Section appears exactly once in
 * display order, and `pinned` lists only Rows that exist, in the order they were
 * pinned.
 */
export interface Arrangement {
  readonly sectionOrder: readonly string[];
  readonly pinned: readonly string[];
}

/** One Row positioned by an {@link Arrangement}; `pinned` is true when it was lifted to the top of its Section. */
export interface ArrangedRow {
  readonly key: string;
  readonly row: SheetRow;
  readonly sectionKey: string;
  readonly pinned: boolean;
}

/** One Section positioned by an {@link Arrangement}, holding every one of its Rows: the pinned first, then the rest. */
export interface ArrangedSection {
  readonly section: SheetSection;
  readonly rows: readonly ArrangedRow[];
}

function uniqueKnown(keys: readonly string[], known: ReadonlySet<string>): string[] {
  const seen = new Set<string>();
  const kept: string[] = [];
  for (const key of keys) {
    if (known.has(key) && !seen.has(key)) {
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

/**
 * Resolves [layout] against [sections]. An absent Layout yields the Template
 * order with nothing pinned. A partial Section list is completed in Template
 * order; stale and duplicate keys are dropped (first occurrence wins); a pin that
 * is no Row is ignored.
 */
export function resolveArrangement(
  sections: readonly SheetSection[],
  layout: Layout | undefined,
): Arrangement {
  const everyRow = new Set(sections.flatMap((section) => section.rows.map((row) => rowKey(row))));
  return {
    sectionOrder: withMissingAppended(
      layout?.sectionOrder ?? [],
      sections.map((section) => section.key),
    ),
    pinned: uniqueKnown(layout?.pinned ?? [], everyRow),
  };
}

function sameList(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((key, index) => key === right[index]);
}

/** True when two arrangements order every Section and every pin identically. */
export function arrangementsEqual(left: Arrangement, right: Arrangement): boolean {
  return sameList(left.sectionOrder, right.sectionOrder) && sameList(left.pinned, right.pinned);
}

/**
 * Serializes [arrangement] to a Layout that deviates from the Template only
 * where the player did: a Section order equal to the Template's is omitted, so a
 * later Template change still shows through. Returns `null` when nothing deviates,
 * which is what a reset to default sends. The arrangement is re-resolved first, so
 * a pin that names no Row never reaches the wire.
 */
export function arrangementToLayout(
  sections: readonly SheetSection[],
  arrangement: Arrangement,
): Layout | null {
  const resolved = resolveArrangement(sections, arrangement);
  const template = resolveArrangement(sections, undefined);
  const sectionOrder = sameList(resolved.sectionOrder, template.sectionOrder)
    ? []
    : resolved.sectionOrder;
  if (sectionOrder.length === 0 && resolved.pinned.length === 0) {
    return null;
  }
  return { sectionOrder, pinned: resolved.pinned };
}

/**
 * Derives what to draw: the Sections in arranged order, each holding all of its
 * Rows — the pinned ones first in pin order, the rest in Template order. A Row
 * never leaves its Section. The arrangement is re-resolved first, so one that
 * predates a Template change still draws every current Section and Row.
 */
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

/** A `config.*` proxy leaf draws a switch whatever its type; the first path segment decides, as in the control. */
function isConfigProxy(path: string): boolean {
  return path.split('.')[0] === 'config';
}

/**
 * True when [row] is a lone Row whose leaf is a number, editable or derived, that draws as a
 * number and not as a switch. Table, exclusive-group and text Rows never qualify.
 */
export function isNumberRow(row: SheetRow): boolean {
  if (row.kind !== 'normal') {
    return false;
  }
  const { path, schema } = row.field;
  return schema.type === 'number' && !(schema.isEditable && isConfigProxy(path));
}
