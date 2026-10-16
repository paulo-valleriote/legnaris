/**
 * The unsaved-draft cadence (WSC-26, WSC-27, R6).
 *
 * A mounted screen starts one cadence and stops it on unmount; every tick asks
 * the caller for the current pending snapshot and writes it through the local
 * port. The ticker is injected, so a suite drives the cadence with a fake and no
 * case ever waits a real minute (AD-008). A clean sheet (no snapshot) and an
 * empty delta both write nothing, and a failing write never blocks editing.
 */

import type { SheetStore } from './sheet_store';
import { bestEffort } from './sheet_store';

/** The draft backup cadence length, in milliseconds (AD-008). */
export const DRAFT_CADENCE_MS = 60_000;

/** The pending draft one tick would write: the sheet and its editable delta (WSC-26). */
export interface DraftSnapshot {
  readonly sheetId: string;
  readonly delta: Record<string, unknown>;
}

/** The injectable ticker: schedules [tick] every [intervalMs] and returns its cancel (R6). */
export type DraftScheduler = (tick: () => void, intervalMs: number) => () => void;

/** The default ticker: a cancellable `setInterval` (R6). */
function defaultScheduler(tick: () => void, intervalMs: number): () => void {
  const timer = setInterval(tick, intervalMs);
  return () => clearInterval(timer);
}

/**
 * Starts the periodic draft backup and returns its stop function (WSC-26, WSC-27).
 *
 * `getDelta` returns the sheet's current pending snapshot, or `null` while the
 * sheet is clean; each tick writes a non-empty snapshot through the port, skips a
 * clean sheet or an empty delta, and swallows a write failure.
 */
export function startDraftCadence(
  store: SheetStore,
  getDelta: () => DraftSnapshot | null,
  scheduler: DraftScheduler = defaultScheduler,
): () => void {
  return scheduler(() => {
    const snapshot = getDelta();
    if (snapshot === null || Object.keys(snapshot.delta).length === 0) {
      return;
    }
    void bestEffort(() => store.writeDraft(snapshot.sheetId, snapshot.delta));
  }, DRAFT_CADENCE_MS);
}
