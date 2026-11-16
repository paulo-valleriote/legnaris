import type { DiceBlock } from '../api/models/dice';
import { parseDiceBlock } from '../api/models/dice';
import type { Layout } from '../api/models/layout';
import { isLayout } from '../api/models/layout';
import type { Sheet } from '../api/models/sheet';
import { hasEntries, isPlainObject } from '../api/parse';

export const SCHEMA_VERSION = 3;

export async function bestEffort(operation: () => Promise<unknown>): Promise<void> {
  try {
    await operation();
  } catch {}
}

export interface CachedSheetRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly sheet: Sheet;
}

export interface DraftRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly delta: Record<string, unknown>;
  readonly dice?: DiceBlock;
}

export interface SheetDraft {
  readonly delta: Record<string, unknown>;
  readonly dice?: DiceBlock;
}

export function hasPendingWork(draft: SheetDraft): boolean {
  return hasEntries(draft.delta) || draft.dice !== undefined;
}

export interface LayoutDraftRecord {
  readonly schemaVersion: typeof SCHEMA_VERSION;
  readonly id: string;
  readonly layout: Layout | null;
}

export interface LayoutDraft {
  readonly layout: Layout | null;
}

export interface SheetStore {
  readSheet(id: string): Promise<Sheet | null>;
  putSheet(sheet: Sheet): Promise<void>;
  deleteSheet(id: string): Promise<void>;
  readDraft(id: string): Promise<SheetDraft | null>;
  writeDraft(id: string, delta: Record<string, unknown>, dice?: DiceBlock): Promise<void>;
  deleteDraft(id: string): Promise<void>;
  readLayoutDraft(id: string): Promise<LayoutDraft | null>;
  writeLayoutDraft(id: string, layout: Layout | null): Promise<void>;
  deleteLayoutDraft(id: string): Promise<void>;
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

export function toCachedSheetRecord(sheet: Sheet): CachedSheetRecord {
  return { schemaVersion: SCHEMA_VERSION, id: sheet.id, sheet };
}

export function toDraftRecord(
  id: string,
  delta: Record<string, unknown>,
  dice?: DiceBlock,
): DraftRecord {
  return dice === undefined
    ? { schemaVersion: SCHEMA_VERSION, id, delta }
    : { schemaVersion: SCHEMA_VERSION, id, delta, dice };
}

export function toLayoutDraftRecord(id: string, layout: Layout | null): LayoutDraftRecord {
  return { schemaVersion: SCHEMA_VERSION, id, layout };
}

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

export interface MemorySheetStore extends SheetStore {
  putRawSheet(id: string, record: unknown): void;
  putRawDraft(id: string, record: unknown): void;
  putRawLayoutDraft(id: string, record: unknown): void;
  records(): unknown[];
}

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
