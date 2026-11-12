/**
 * The local persistence port for the sheet cycle (WSC-11, WSC-26 … WSC-31).
 *
 * Two versioned records live behind one typed boundary: the offline cache of a
 * loaded or created sheet and the unsaved draft of a sheet's pending delta and
 * pending Dice block (ADR 0006). The
 * app builds the `idb`-backed adapter and every suite injects the in-memory
 * double, so the versioning and corruption rules are testable without a
 * database. No operation stores a credential (WSC-31).
 */

import type { DiceBlock } from '../api/models/dice';
import { parseDiceBlock } from '../api/models/dice';
import type { Layout } from '../api/models/layout';
import { isLayout } from '../api/models/layout';
import type { Sheet } from '../api/models/sheet';
import { isPlainObject } from '../api/parse';

/**
 * The schema version this build writes and understands (WSC-30). Version 2 added the
 * Sheet's committed `layout` to the cache record and the pending-arrangement record;
 * version 3 dropped the per-Section Row order from that Layout. An older record reads
 * as missing, so the local stores are rebuilt rather than migrated.
 */
export const SCHEMA_VERSION = 3;

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

/**
 * The versioned draft record of one sheet's pending editable delta (WSC-26) and,
 * when one is pending, its full Dice block (ADR 0006). A record written before
 * the block existed has no `dice` and stays valid.
 */
export interface DraftRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly delta: Record<string, unknown>;
  readonly dice?: DiceBlock;
}

/** A draft read back from the store: the pending delta and, when one was pending, the Dice block. */
export interface SheetDraft {
  readonly delta: Record<string, unknown>;
  readonly dice?: DiceBlock;
}

/** The versioned record of a sheet's pending, uncommitted arrangement; `null` stages a reset to default. */
export interface LayoutDraftRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly layout: Layout | null;
}

/** A pending arrangement read back from the store; `layout: null` is a staged reset to default. */
export interface LayoutDraft {
  readonly layout: Layout | null;
}

/** The persistence boundary every sheet/library module depends on (WSC-11, WSC-26). */
export interface SheetStore {
  /** Reads the cached sheet for [id], or `null` (WSC-30). */
  readSheet(id: string): Promise<Sheet | null>;
  /** Writes the cache record for [sheet], keyed by its identifier (WSC-11). */
  putSheet(sheet: Sheet): Promise<void>;
  /** Deletes the cached sheet for [id], best-effort (WSC-06). */
  deleteSheet(id: string): Promise<void>;
  /** Reads the draft for [id], or `null` (WSC-30). */
  readDraft(id: string): Promise<SheetDraft | null>;
  /** Writes the pending editable delta for [id] (WSC-26). */
  writeDraft(id: string, delta: Record<string, unknown>, dice?: DiceBlock): Promise<void>;
  /** Deletes the draft for [id], best-effort (WSC-29). */
  deleteDraft(id: string): Promise<void>;
  /** Reads the pending arrangement for [id], or `null` when none is stored. */
  readLayoutDraft(id: string): Promise<LayoutDraft | null>;
  /** Writes the pending arrangement for [id]; a `null` layout stages a reset to default. */
  writeLayoutDraft(id: string, layout: Layout | null): Promise<void>;
  /** Deletes the pending arrangement for [id], best-effort. */
  deleteLayoutDraft(id: string): Promise<void>;
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
    (value['layout'] === undefined || isLayout(value['layout'])) &&
    typeof value['createdAt'] === 'string' &&
    typeof value['updatedAt'] === 'string'
  );
}

/** Wraps [sheet] in the cache envelope the store writes (WSC-11). */
export function toCachedSheetRecord(sheet: Sheet): CachedSheetRecord {
  return { schemaVersion: SCHEMA_VERSION, id: sheet.id, sheet };
}

/** Wraps [delta] and an optional pending Dice block in the draft envelope the store writes (WSC-26). */
export function toDraftRecord(
  id: string,
  delta: Record<string, unknown>,
  dice?: DiceBlock,
): DraftRecord {
  return dice === undefined
    ? { schemaVersion: SCHEMA_VERSION, id, delta }
    : { schemaVersion: SCHEMA_VERSION, id, delta, dice };
}

/** Wraps a pending arrangement in the envelope the store writes. */
export function toLayoutDraftRecord(id: string, layout: Layout | null): LayoutDraftRecord {
  return { schemaVersion: SCHEMA_VERSION, id, layout };
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
 * Resolves a stored draft record to its delta and Dice block, or `null`.
 *
 * The same total rule as {@link readSheetRecord}: a missing, malformed,
 * unsupported-version or foreign record reads as `null` (WSC-30). A `dice` that
 * is not an object is dropped and the delta still restores.
 */
export function readDraftRecord(record: unknown, id: string): SheetDraft | null {
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
    if (!isPlainObject(delta)) {
      return null;
    }
    const dice = record['dice'];
    return isPlainObject(dice) ? { delta, dice: parseDiceBlock(dice) } : { delta };
  } catch {
    return null;
  }
}

/**
 * Resolves a stored pending-arrangement record, or `null`.
 *
 * The same total rule as {@link readSheetRecord}: a missing, malformed,
 * unsupported-version or foreign record, or one whose layout is neither `null` nor
 * a well-formed Layout, reads as `null` — never a throw.
 */
export function readLayoutDraftRecord(record: unknown, id: string): LayoutDraft | null {
  try {
    if (!isPlainObject(record)) {
      return null;
    }
    if (record['schemaVersion'] !== SCHEMA_VERSION || record['id'] !== id) {
      return null;
    }
    const layout = record['layout'];
    if (layout === null) {
      return { layout: null };
    }
    return isLayout(layout) ? { layout } : null;
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
    writeDraft: (id, delta, dice) => resolve().then((store) => store.writeDraft(id, delta, dice)),
    deleteDraft: (id) => bestEffort(() => resolve().then((store) => store.deleteDraft(id))),
    readLayoutDraft: (id) => resolve().then((store) => store.readLayoutDraft(id)),
    writeLayoutDraft: (id, layout) => resolve().then((store) => store.writeLayoutDraft(id, layout)),
    deleteLayoutDraft: (id) =>
      bestEffort(() => resolve().then((store) => store.deleteLayoutDraft(id))),
    clearAll: () => bestEffort(() => resolve().then((store) => store.clearAll())),
  };
}

/** The in-memory double every logic and screen suite injects (R1). */
export interface MemorySheetStore extends SheetStore {
  /** Test seam: writes a raw cache record verbatim, so corruption is exercisable. */
  putRawSheet(id: string, record: unknown): void;
  /** Test seam: writes a raw draft record verbatim, so corruption is exercisable. */
  putRawDraft(id: string, record: unknown): void;
  /** Test seam: writes a raw pending-arrangement record verbatim, so corruption is exercisable. */
  putRawLayoutDraft(id: string, record: unknown): void;
  /** Every stored record, so a suite can assert nothing credential-shaped is held (WSC-31). */
  records(): unknown[];
}

/** Builds the `Map`-backed double the suites inject through `AppProviders` (R1). */
export function createMemorySheetStore(): MemorySheetStore {
  const sheets = new Map<string, unknown>();
  const drafts = new Map<string, unknown>();
  const layoutDrafts = new Map<string, unknown>();

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
    writeDraft: (id, delta, dice) => {
      drafts.set(id, toDraftRecord(id, delta, dice));
      return Promise.resolve();
    },
    deleteDraft: (id) => {
      drafts.delete(id);
      return Promise.resolve();
    },
    readLayoutDraft: (id) => Promise.resolve(readLayoutDraftRecord(layoutDrafts.get(id), id)),
    writeLayoutDraft: (id, layout) => {
      layoutDrafts.set(id, toLayoutDraftRecord(id, layout));
      return Promise.resolve();
    },
    deleteLayoutDraft: (id) => {
      layoutDrafts.delete(id);
      return Promise.resolve();
    },
    clearAll: () => {
      sheets.clear();
      drafts.clear();
      layoutDrafts.clear();
      return Promise.resolve();
    },
    putRawSheet: (id, record) => {
      sheets.set(id, record);
    },
    putRawDraft: (id, record) => {
      drafts.set(id, record);
    },
    putRawLayoutDraft: (id, record) => {
      layoutDrafts.set(id, record);
    },
    records: () => [...sheets.values(), ...drafts.values(), ...layoutDrafts.values()],
  };
}
