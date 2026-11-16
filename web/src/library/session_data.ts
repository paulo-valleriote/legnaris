import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

export function clearSessionData(store: SheetStore): Promise<void> {
  return bestEffort(() => store.clearAll());
}
