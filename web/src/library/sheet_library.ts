import type { Sheet } from '../api/models/sheet';
import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

export function cacheSheet(store: SheetStore, sheet: Sheet): Promise<void> {
  return bestEffort(() => store.putSheet(sheet));
}

export function evictSheet(store: SheetStore, sheetId: string): Promise<void> {
  return Promise.all([
    bestEffort(() => store.deleteSheet(sheetId)),
    bestEffort(() => store.deleteDraft(sheetId)),
    bestEffort(() => store.deleteLayoutDraft(sheetId)),
  ]).then(() => undefined);
}

export function discardDraft(store: SheetStore, sheetId: string): Promise<void> {
  return bestEffort(() => store.deleteDraft(sheetId));
}

export function discardLayoutDraft(store: SheetStore, sheetId: string): Promise<void> {
  return bestEffort(() => store.deleteLayoutDraft(sheetId));
}
