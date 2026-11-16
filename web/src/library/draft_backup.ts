import type { DiceBlock } from '../api/models/dice';
import type { SheetStore } from './sheet_store';
import { bestEffort, hasPendingWork } from './sheet_store';

export const DRAFT_CADENCE_MS = 60_000;

export interface DraftDelta {
  readonly sheetId: string;
  readonly delta: Record<string, unknown>;
  readonly dice?: DiceBlock;
}

export type CadenceScheduler = (tick: () => void, intervalMs: number) => () => void;

const defaultScheduler: CadenceScheduler = (tick, intervalMs) => {
  const handle = setInterval(tick, intervalMs);
  return () => {
    clearInterval(handle);
  };
};

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
