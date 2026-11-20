import { valueAtPath } from '../../api/models/sheet';
import { message } from '../../l10n/strings';

export interface PendingChange {
  readonly path: string;
  readonly before: string;
  readonly after: string;
}

export function displayValue(value: unknown): string {
  if (value === null || value === undefined) {
    return message('sheet.pending.emptyValue');
  }
  if (typeof value === 'string') {
    return value.trim() === '' ? message('sheet.pending.emptyValue') : value;
  }
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value);
  }
  return message('sheet.pending.emptyValue');
}

export function pendingChanges(
  defaults: Record<string, unknown>,
  data: Record<string, unknown>,
  editedPaths: Iterable<string>,
): PendingChange[] {
  const changes: PendingChange[] = [];
  for (const path of editedPaths) {
    const before = valueAtPath(defaults, path);
    const after = valueAtPath(data, path);
    if (Object.is(before, after)) {
      continue;
    }
    changes.push({ path, before: displayValue(before), after: displayValue(after) });
  }
  return changes;
}
