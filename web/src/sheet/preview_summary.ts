import { rowFieldPaths } from '../engine/field_grouping';
import type { ArrangedSection } from '../engine/layout';
import { message, messageWith } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';
import type { PreviewMarks } from './sheet_edit';
import { markForPaths } from './sheet_edit';

export function recalculatedCount(marks: PreviewMarks): number {
  let count = 0;
  for (const path of marks.recalculated) {
    if (!marks.modified.has(path)) {
      count += 1;
    }
  }
  return count;
}

export function recalculatedSummary(count: number): string {
  const key: MessageKey =
    count === 0
      ? 'sheet.recalculatedNone'
      : count === 1
        ? 'sheet.recalculatedOne'
        : 'sheet.recalculatedMany';
  return messageWith(key, { count });
}

export function footerStatus(pending: boolean, count: number | undefined): string {
  if (count === undefined) {
    return pending ? message('sheet.pendingEdits') : '';
  }
  if (!pending) {
    return recalculatedSummary(count);
  }
  return count === 0
    ? message('sheet.statusUnsavedNone')
    : messageWith('sheet.statusUnsavedCount', { count });
}

export function markedRowCounts(
  view: readonly ArrangedSection[],
  marks: PreviewMarks | undefined,
): ReadonlyMap<string, number> {
  const counts = new Map<string, number>();
  if (marks === undefined) {
    return counts;
  }
  for (const entry of view) {
    const count = entry.rows.filter(
      (row) => markForPaths(marks, rowFieldPaths(row.row)) !== undefined,
    ).length;
    if (count > 0) {
      counts.set(entry.section.key, count);
    }
  }
  return counts;
}
