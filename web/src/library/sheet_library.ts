/**
 * The offline cache helpers on top of the `SheetStore` port (WSC-06, WSC-11,
 * WSC-29, R1). Every operation is best-effort: a failing local write or eviction
 * never blocks the flow that triggered it, and an eviction failure never hides a
 * rendered message (WSC-06).
 */

import type { Sheet } from '../api/models/sheet';
import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

/** Writes a loaded or created sheet to the local cache by identifier (WSC-11). */
export function cacheSheet(store: SheetStore, sheet: Sheet): Promise<void> {
  return bestEffort(() => store.putSheet(sheet));
}

/** Reads the cached sheet for [id], or `null` (WSC-11). */
export async function readCachedSheet(store: SheetStore, id: string): Promise<Sheet | null> {
  try {
    return await store.readSheet(id);
  } catch {
    return null;
  }
}

/**
 * Evicts a sheet and its draft from local storage, best-effort (WSC-06, WSC-29).
 *
 * Used when the open resolves that the sheet no longer exists; a failing
 * eviction never changes the rendered state.
 */
export async function evictSheet(store: SheetStore, id: string): Promise<void> {
  await Promise.all([
    bestEffort(() => store.deleteSheet(id)),
    bestEffort(() => store.deleteDraft(id)),
  ]);
}
