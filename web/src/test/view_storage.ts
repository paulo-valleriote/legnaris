import type { ViewStorage } from '../sheet/view_store';

/** An in-memory {@link ViewStorage} whose records a suite can read and seed. */
export interface MemoryViewStorage extends ViewStorage {
  readonly records: Map<string, string>;
}

/** Builds a storage double; [seed] preloads records, and a throwing flag makes every call refuse. */
export function createMemoryViewStorage(
  seed: Record<string, string> = {},
  options: { readonly throws?: boolean } = {},
): MemoryViewStorage {
  const records = new Map(Object.entries(seed));
  return {
    records,
    getItem: (key) => {
      if (options.throws === true) {
        throw new Error('storage unavailable');
      }
      return records.get(key) ?? null;
    },
    setItem: (key, value) => {
      if (options.throws === true) {
        throw new Error('storage unavailable');
      }
      records.set(key, value);
    },
  };
}
