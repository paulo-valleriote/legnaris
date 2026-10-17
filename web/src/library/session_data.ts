/**
 * The session-local sweep (WAS-19 … WAS-22, R6).
 *
 * `AD-010` is the account-switch hazard: the local data of a finished session
 * must never be visible to the next one. Every session-local record lives behind
 * the F5 `SheetStore` (`sheets` + `drafts`), so the sweep is one clear-all over
 * that port. It is best-effort — a failing store never blocks the session end —
 * and it is called only at a definitive boundary, never on a transient refresh
 * failure, which must keep the data (WAS-05, WAS-21).
 */

import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

/** Removes every cached sheet and every draft of the current session (WAS-21). */
export function clearSessionData(store: SheetStore): Promise<void> {
  return bestEffort(() => store.clearAll());
}
