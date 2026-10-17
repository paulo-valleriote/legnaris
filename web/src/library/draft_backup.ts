import { openLibrary, DraftEntry } from './sheet_library';

// PWA-04: draft backup with injectable cadence; restores only editable leaves.
export interface DraftCadence {
  intervalMs: number;
  onBackup: (entry: DraftEntry) => Promise<void>;
}

export function createDraftCadence(cadence: DraftCadence) {
  return {
    schedule(entry: DraftEntry) {
      setInterval(async () => {
        await cadence.onBackup(entry);
      }, cadence.intervalMs);
    },
  };
}
