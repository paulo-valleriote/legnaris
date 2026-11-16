import type { ViewStorage } from '../sheet/view_store';

export interface MemoryViewStorage extends ViewStorage {
  readonly records: Map<string, string>;
}

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
