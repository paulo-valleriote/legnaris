/**
 * The periodic draft backup (WSC-26, WSC-27, AD-008).
 *
 * While an edit is pending, one injected tick writes the current editable delta
 * through the sheet-store port. The cadence is pending-only — a clean sheet and
 * an empty delta write nothing — the scheduler is injected so the cadence is
 * exercised without waiting a real interval, and a failed write is swallowed:
 * a backup that cannot land never blocks editing. The returned function stops
 * the cadence on unmount.
 */

import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

/** The default backup cadence, matching the desktop draft interval (AD-008). */
export const DRAFT_CADENCE_MS = 60_000;

/** The pending delta a tick should write, or `null` while there is nothing to back up. */
export interface DraftDelta {
  readonly sheetId: string;
  readonly delta: Record<string, unknown>;
}

/** The injectable tick source; returns the stop function. */
export type CadenceScheduler = (tick: () => void, intervalMs: number) => () => void;

const defaultScheduler: CadenceScheduler = (tick, intervalMs) => {
  const handle = setInterval(tick, intervalMs);
  return () => {
    clearInterval(handle);
  };
};

function hasEntries(delta: Record<string, unknown>): boolean {
  return Object.keys(delta).length > 0;
}

/**
 * Starts the periodic draft backup and returns the function that stops it
 * (WSC-26, WSC-27).
 *
 * Each tick reads the pending delta through [readDelta]; it writes nothing while
 * the reader reports `null` or an empty delta, and otherwise stores the delta
 * best-effort.
 */
export function startDraftCadence(
  store: SheetStore,
  readDelta: () => DraftDelta | null,
  scheduler: CadenceScheduler = defaultScheduler,
): () => void {
  return scheduler(() => {
    const pending = readDelta();
    if (pending === null || !hasEntries(pending.delta)) {
      return;
    }
    void bestEffort(() => store.writeDraft(pending.sheetId, pending.delta));
  }, DRAFT_CADENCE_MS);
}
