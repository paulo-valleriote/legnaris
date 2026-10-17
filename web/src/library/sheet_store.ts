/**
 * The local persistence port for the sheet cycle (WSC-11, WSC-26 … WSC-31).
 *
 * Two versioned records live behind one typed boundary: the offline cache of a
 * loaded or created sheet and the unsaved draft of a sheet's pending delta. The
 * app builds the `idb`-backed adapter and every suite injects the in-memory
 * double, so the versioning and corruption rules are testable without a
 * database. No operation stores a credential (WSC-31).
 */

import type { Sheet } from '../api/models/sheet';
import { isPlainObject } from '../api/parse';

/** The schema version this build writes and understands (WSC-30). */
export const SCHEMA_VERSION = 1;

/**
 * Runs [operation] and swallows any failure.
 *
 * Every local-record write and delete in the feature is best-effort: a failing
 * cache write or eviction never blocks the flow that triggered it (WSC-06).
 */
export async function bestEffort(operation: () => Promise<unknown>): Promise<void> {
  try {
    await operation();
  } catch {
    // A local record is an offline optimisation; a failure must not surface.
  }
}

/** The versioned cache record of one sheet, keyed by its identifier (WSC-11). */
export interface CachedSheetRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly sheet: Sheet;
}

/** The versioned draft record of one sheet's pending editable delta (WSC-26). */
export interface DraftRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly delta: Record<string, unknown>;
}

/** The persistence boundary every sheet/library module depends on (WSC-11, WSC-26). */
export interface SheetStore {
  /** Reads the cached sheet for [id], or `null` (WSC-30). */
  readSheet(id: string): Promise<Sheet | null>;
  /** Writes the cache record for [sheet], keyed by its identifier (WSC-11). */
  putSheet(sheet: Sheet): Promise<void>;
  /** Deletes the cached sheet for [id], best-effort (WSC-06). */
  deleteSheet(id: string): Promise<void>;
  /** Reads the draft delta for [id], or `null` (WSC-30). */
  readDraft(id: string): Promise<Record<string, unknown> | null>;
  /** Writes the pending editable delta for [id] (WSC-26). */
  writeDraft(id: string, delta: Record<string, unknown>): Promise<void>;
  /** Deletes the draft for [id], best-effort (WSC-29). */
  deleteDraft(id: string): Promise<void>;
  /** Deletes every cached sheet and every draft at a session boundary (WAS-21). */
  clearAll(): Promise<void>;
}

function isSheet(value: unknown): value is Sheet {
  if (!isPlainObject(value)) {
    return false;
  }
  return (
    typeof value['id'] === 'string' &&
    typeof value['templateId'] === 'string' &&
    typeof value['name'] === 'string' &&
    isPlainObject(value['data']) &&
    typeof value['createdAt'] === 'string' &&
    typeof value['updatedAt'] === 'string'
  );
}

/** Wraps [sheet] in the cache envelope the store writes (WSC-11). */
export function toCachedSheetRecord(sheet: Sheet): CachedSheetRecord {
  return { schemaVersion: SCHEMA_VERSION, id: sheet.id, sheet };
}

/** Wraps [delta] in the draft envelope the store writes (WSC-26). */
export function toDraftRecord(id: string, delta: Record<string, unknown>): DraftRecord {
  return { schemaVersion: SCHEMA_VERSION, id, delta };
}

/**
 * Resolves a stored cache record to its sheet, or `null`.
 *
 * A record that is missing, is not a well-formed envelope, carries an
 * unsupported `schemaVersion`, or names another [id] reads as `null` — never a
 * throw (WSC-30).
 */
export function readSheetRecord(record: unknown, id: string): Sheet | null {
  try {
    if (!isPlainObject(record)) {
      return null;
    }
    if (record['schemaVersion'] !== SCHEMA_VERSION) {
      return null;
    }
    if (record['id'] !== id) {
      return null;
    }
    const sheet = record['sheet'];
    return isSheet(sheet) && sheet.id === id ? sheet : null;
  } catch {
    return null;
  }
}

/**
 * Resolves a stored draft record to its delta, or `null`.
 *
 * The same total rule as {@link readSheetRecord}: a missing, malformed,
 * unsupported-version or foreign record reads as `null` (WSC-30).
 */
export function readDraftRecord(record: unknown, id: string): Record<string, unknown> | null {
  try {
    if (!isPlainObject(record)) {
      return null;
    }
    if (record['schemaVersion'] !== SCHEMA_VERSION) {
      return null;
    }
    if (record['id'] !== id) {
      return null;
    }
    const delta = record['delta'];
    return isPlainObject(delta) ? delta : null;
  } catch {
    return null;
  }
}

/**
 * Wraps an asynchronous store opening in a synchronous {@link SheetStore}.
 *
 * The `idb` adapter opens its database asynchronously while React context is
 * built synchronously; this adapter resolves the opening once on first use (and
 * retries after a failed opening), so a consumer sees one total `SheetStore`
 * (R5).
 */
export function createLazySheetStore(open: () => Promise<SheetStore>): SheetStore {
  let pending: Promise<SheetStore> | undefined;

  const resolve = (): Promise<SheetStore> => {
    if (pending === undefined) {
      pending = open().catch((error: unknown) => {
        pending = undefined;
        throw error;
      });
    }
    return pending;
  };

  return {
    readSheet: (id) => resolve().then((store) => store.readSheet(id)),
    putSheet: (sheet) => resolve().then((store) => store.putSheet(sheet)),
    deleteSheet: (id) => bestEffort(() => resolve().then((store) => store.deleteSheet(id))),
    readDraft: (id) => resolve().then((store) => store.readDraft(id)),
    writeDraft: (id, delta) => resolve().then((store) => store.writeDraft(id, delta)),
    deleteDraft: (id) => bestEffort(() => resolve().then((store) => store.deleteDraft(id))),
    clearAll: () => bestEffort(() => resolve().then((store) => store.clearAll())),
  };
}

/** The in-memory double every logic and screen suite injects (R1). */
export interface MemorySheetStore extends SheetStore {
  /** Test seam: writes a raw cache record verbatim, so corruption is exercisable. */
  putRawSheet(id: string, record: unknown): void;
  /** Test seam: writes a raw draft record verbatim, so corruption is exercisable. */
  putRawDraft(id: string, record: unknown): void;
  /** Every stored record, so a suite can assert nothing credential-shaped is held (WSC-31). */
  records(): unknown[];
}

/** Builds the `Map`-backed double the suites inject through `AppProviders` (R1). */
export function createMemorySheetStore(): MemorySheetStore {
  const sheets = new Map<string, unknown>();
  const drafts = new Map<string, unknown>();

  return {
    readSheet: (id) => Promise.resolve(readSheetRecord(sheets.get(id), id)),
    putSheet: (sheet) => {
      sheets.set(sheet.id, toCachedSheetRecord(sheet));
      return Promise.resolve();
    },
    deleteSheet: (id) => {
      sheets.delete(id);
      return Promise.resolve();
    },
    readDraft: (id) => Promise.resolve(readDraftRecord(drafts.get(id), id)),
    writeDraft: (id, delta) => {
      drafts.set(id, toDraftRecord(id, delta));
      return Promise.resolve();
    },
    deleteDraft: (id) => {
      drafts.delete(id);
      return Promise.resolve();
    },
    clearAll: () => {
      sheets.clear();
      drafts.clear();
      return Promise.resolve();
    },
    putRawSheet: (id, record) => {
      sheets.set(id, record);
    },
    putRawDraft: (id, record) => {
      drafts.set(id, record);
    },
    records: () => [...sheets.values(), ...drafts.values()],
  };
}
