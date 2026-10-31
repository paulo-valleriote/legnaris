/**
 * Cache helpers over the sheet-store port (WSC-06, WSC-11, WSC-29).
 *
 * The local record itself lives behind the F5 `SheetStore`
 * (`sheet_store.ts` / `idb_sheet_store.ts`); these helpers only name the flows
 * that touch it — caching a loaded sheet, evicting a missing sheet together
 * with its draft, and discarding a draft. Every one is best-effort: a local
 * record is an offline optimisation and a failure must never block the flow
 * that triggered it (WSC-06).
 */

import type { Sheet } from '../api/models/sheet';
import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

/** Caches a loaded or created sheet by its identifier (WSC-11). */
export function cacheSheet(store: SheetStore, sheet: Sheet): Promise<void> {
  return bestEffort(() => store.putSheet(sheet));
}

/** Evicts a missing sheet, its draft and its pending arrangement (WSC-06, WSC-29). */
export function evictSheet(store: SheetStore, sheetId: string): Promise<void> {
  return Promise.all([
    bestEffort(() => store.deleteSheet(sheetId)),
    bestEffort(() => store.deleteDraft(sheetId)),
    bestEffort(() => store.deleteLayoutDraft(sheetId)),
  ]).then(() => undefined);
}

/** Discards the draft of an abandoned edit (WSC-29). */
export function discardDraft(store: SheetStore, sheetId: string): Promise<void> {
  return bestEffort(() => store.deleteDraft(sheetId));
}

/** Discards the pending arrangement of an abandoned Arrange session. */
export function discardLayoutDraft(store: SheetStore, sheetId: string): Promise<void> {
  return bestEffort(() => store.deleteLayoutDraft(sheetId));
}
