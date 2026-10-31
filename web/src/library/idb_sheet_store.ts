/**
 * The `idb`-backed `SheetStore` — the only module in the feature that imports
 * `idb` (R5). One database holds the three object stores, `sheets`, `drafts` and `layoutDrafts`,
 * each record the versioned envelope `sheet_store.ts` defines; the adapter
 * reuses the same read rules so a corrupt, unsupported-version or foreign record
 * reads as `null` exactly as the in-memory double does (WSC-30).
 */

import type { DBSchema, IDBPDatabase } from 'idb';
import { wrap } from 'idb';
import type { Sheet } from '../api/models/sheet';
import type { SheetStore } from './sheet_store';
import {
  SCHEMA_VERSION,
  readDraftRecord,
  readLayoutDraftRecord,
  readSheetRecord,
  toCachedSheetRecord,
  toDraftRecord,
  toLayoutDraftRecord,
} from './sheet_store';

const DATABASE_NAME = 'legnaris-sheets';

/** The database name the adapter opens; exported so an adapter suite can seed raw records. */
export const SHEET_DATABASE_NAME = DATABASE_NAME;

interface SheetStoreSchema extends DBSchema {
  sheets: { key: string; value: unknown };
  drafts: { key: string; value: unknown };
  layoutDrafts: { key: string; value: unknown };
}

async function readOrNull<T>(read: () => Promise<T | null>): Promise<T | null> {
  try {
    return await read();
  } catch {
    return null;
  }
}

async function bestEffort(write: () => Promise<unknown>): Promise<void> {
  try {
    await write();
  } catch {
    // A failing delete or backup never blocks the flow that triggered it.
  }
}

/**
 * Opens the database through [factory] and upgrades it to {@link SCHEMA_VERSION}.
 *
 * `idb`'s `openDB` reads the global `indexedDB` and cannot take an injected
 * factory, so the opening is driven here: the raw request carries the
 * `upgradeneeded` listener and `idb`'s `wrap` yields the same typed database the
 * `openDB` helper would (R5).
 */
async function openDatabase(
  factory: IDBFactory,
  name: string,
  version: number,
): Promise<IDBPDatabase<SheetStoreSchema>> {
  const request = factory.open(name, version);
  request.addEventListener('upgradeneeded', () => {
    const database = request.result;
    if (!database.objectStoreNames.contains('sheets')) {
      database.createObjectStore('sheets');
    }
    if (!database.objectStoreNames.contains('drafts')) {
      database.createObjectStore('drafts');
    }
    if (!database.objectStoreNames.contains('layoutDrafts')) {
      database.createObjectStore('layoutDrafts');
    }
  });
  const database = (await wrap(request)) as IDBPDatabase<SheetStoreSchema> | undefined;
  if (database === undefined) {
    throw new Error('IndexedDB closed before the sheet store database opened.');
  }
  return database;
}

/**
 * Opens the database and returns the `idb` adapter (WSC-11, WSC-30).
 *
 * The factory is injected so the adapter is exercised under `fake-indexeddb`
 * (jsdom ships no IndexedDB); the app passes the browser's `indexedDB`.
 */
export async function createIdbSheetStore(factory: IDBFactory = indexedDB): Promise<SheetStore> {
  const db = await openDatabase(factory, DATABASE_NAME, SCHEMA_VERSION);

  return {
    readSheet: (id) => readOrNull(async () => readSheetRecord(await db.get('sheets', id), id)),
    putSheet: async (sheet: Sheet) => {
      await db.put('sheets', toCachedSheetRecord(sheet), sheet.id);
    },
    deleteSheet: (id) => bestEffort(() => db.delete('sheets', id)),
    readDraft: (id) => readOrNull(async () => readDraftRecord(await db.get('drafts', id), id)),
    writeDraft: async (id, delta) => {
      await db.put('drafts', toDraftRecord(id, delta), id);
    },
    deleteDraft: (id) => bestEffort(() => db.delete('drafts', id)),
    readLayoutDraft: (id) =>
      readOrNull(async () => readLayoutDraftRecord(await db.get('layoutDrafts', id), id)),
    writeLayoutDraft: async (id, layout) => {
      await db.put('layoutDrafts', toLayoutDraftRecord(id, layout), id);
    },
    deleteLayoutDraft: (id) => bestEffort(() => db.delete('layoutDrafts', id)),
    clearAll: () =>
      bestEffort(async () => {
        await db.clear('sheets');
        await db.clear('drafts');
        await db.clear('layoutDrafts');
      }),
  };
}
