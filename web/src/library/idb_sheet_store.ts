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
  } catch {}
}

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

export async function createIdbSheetStore(factory: IDBFactory = indexedDB): Promise<SheetStore> {
  const db = await openDatabase(factory, DATABASE_NAME, SCHEMA_VERSION);

  return {
    readSheet: (id) => readOrNull(async () => readSheetRecord(await db.get('sheets', id), id)),
    putSheet: async (sheet: Sheet) => {
      await db.put('sheets', toCachedSheetRecord(sheet), sheet.id);
    },
    deleteSheet: (id) => bestEffort(() => db.delete('sheets', id)),
    readDraft: (id) => readOrNull(async () => readDraftRecord(await db.get('drafts', id), id)),
    writeDraft: async (id, delta, dice) => {
      await db.put('drafts', toDraftRecord(id, delta, dice), id);
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
