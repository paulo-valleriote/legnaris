/**
 * The periodic draft backup (WSC-26, WSC-27, AD-008).
 *
 * While an edit is pending, one injected tick writes the current editable delta,
 * and the pending Dice block when there is one, through the sheet-store port. The
 * cadence is pending-only — a clean sheet, or an empty delta with no Dice block,
 * writes nothing — the scheduler is injected so the cadence is
 * exercised without waiting a real interval, and a failed write is swallowed:
 * a backup that cannot land never blocks editing. The returned function stops
 * the cadence on unmount.
 */

import type { DiceBlock } from '../api/models/dice';
import type { SheetStore } from './sheet_store';
import { bestEffort, hasPendingWork } from './sheet_store';

/** The default backup cadence, matching the desktop draft interval (AD-008). */
export const DRAFT_CADENCE_MS = 60_000;

/** The pending work a tick should write, or `null` while there is nothing to back up. */
export interface DraftDelta {
  readonly sheetId: string;
  readonly delta: Record<string, unknown>;
  /** The full Dice block, present only while it differs from the saved one (ADR 0006). */
  readonly dice?: DiceBlock;
}

/** The injectable tick source; returns the stop function. */
export type CadenceScheduler = (tick: () => void, intervalMs: number) => () => void;

const defaultScheduler: CadenceScheduler = (tick, intervalMs) => {
  const handle = setInterval(tick, intervalMs);
  return () => {
    clearInterval(handle);
  };
};

/**
 * Starts the periodic draft backup and returns the function that stops it
 * (WSC-26, WSC-27).
 *
 * Each tick reads the pending work through [readDelta]; it writes nothing while
 * the reader reports `null`, or an empty delta with no Dice block, and otherwise
 * stores both best-effort.
 */
export function startDraftCadence(
  store: SheetStore,
  readDelta: () => DraftDelta | null,
  scheduler: CadenceScheduler = defaultScheduler,
): () => void {
  return scheduler(() => {
    const pending = readDelta();
    if (pending === null || !hasPendingWork(pending)) {
      return;
    }
    void bestEffort(() => store.writeDraft(pending.sheetId, pending.delta, pending.dice));
  }, DRAFT_CADENCE_MS);
}
