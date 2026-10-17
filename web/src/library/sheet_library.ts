import { openDB, DBSchema, IDBPDatabase } from 'idb';

// PWA-02 / PWA-05: versioned IndexedDB library; never stores tokens.
export interface DraftEntry {
  template_id: string;
  sheet_id: string | null;
  data: Partial<Record<string, unknown>>;
  editable_filter: boolean;
  timestamp: number;
}

export interface SheetLibraryDB extends DBSchema {
  drafts: {
    key: string;
    value: DraftEntry;
    indexes: { 'by-template': string; 'by-sheet': string | null };
  };
}

export async function openLibrary(version: number = 1): Promise<IDBPDatabase<SheetLibraryDB>> {
  return openDB<SheetLibraryDB>('legnaris-library', version, {
    upgrade(db) {
      const store = db.createObjectStore('drafts', { keyPath: 'key' });
      store.createIndex('by-template', 'template_id');
      store.createIndex('by-sheet', 'sheet_id');
    },
  });
}

export async function evictSheet(
  templateId: string,
  sheetId: string | null,
  db?: IDBPDatabase<SheetLibraryDB>,
) {
  const instance = db ?? (await openLibrary());
  const all = await instance.getAllFromIndex('drafts', 'by-template', templateId);
  const matches = all.filter((d) => d.sheet_id === sheetId);
  for (const m of matches) {
    await instance.delete('drafts', m.key);
  }
}

export async function saveDraft(entry: DraftEntry, db?: IDBPDatabase<SheetLibraryDB>) {
  const instance = db ?? (await openLibrary());
  await instance.put('drafts', {
    ...entry,
    key: `${entry.template_id}:${entry.sheet_id ?? 'new'}`,
  });
}

export async function getDraft(
  templateId: string,
  sheetId?: string | null,
  db?: IDBPDatabase<SheetLibraryDB>,
) {
  const instance = db ?? (await openLibrary());
  const all = await instance.getAllFromIndex('drafts', 'by-template', templateId);
  if (!sheetId) return all;
  return all.find((d) => d.sheet_id === sheetId) ?? null;
}
