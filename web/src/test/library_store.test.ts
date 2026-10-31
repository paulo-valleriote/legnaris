import 'fake-indexeddb/auto';
import { describe, expect, it } from 'vitest';
import type { IDBPDatabase } from 'idb';
import { wrap } from 'idb';
import { IDBFactory as FDBFactory } from 'fake-indexeddb';
import type { Sheet } from '../api/models/sheet';
import { createIdbSheetStore, SHEET_DATABASE_NAME } from '../library/idb_sheet_store';
import type { SheetStore } from '../library/sheet_store';
import {
  SCHEMA_VERSION,
  createLazySheetStore,
  createMemorySheetStore,
} from '../library/sheet_store';

const SHEET: Sheet = {
  id: '01a0f967-f42b-710e-851f-8f293a634921',
  templateId: 'fantasy_v1',
  name: 'Aragorn',
  data: { stats: { dexterity: 16 } },
  createdAt: '2026-10-01T21:38:54.635Z',
  updatedAt: '2026-10-02T09:00:00.000Z',
};

const OTHER_ID = '01a0f967-f42b-710e-851f-8f293a634922';

interface AdapterHarness {
  readonly store: SheetStore;
  readonly db: IDBPDatabase;
}

async function idbHarness(): Promise<AdapterHarness> {
  const factory = new FDBFactory();
  const store = await createIdbSheetStore(factory);
  const db = (await wrap(factory.open(SHEET_DATABASE_NAME, SCHEMA_VERSION))) as IDBPDatabase;
  return { store, db };
}

function credentialFree(records: readonly unknown[]): boolean {
  const text = JSON.stringify(records);
  return !/token|password|secret|authorization|cookie|credential/i.test(text);
}

describe('sheet store: cache envelope (WSC-11, WSC-30, WSC-31)', () => {
  it('Edge: readSheet resolves null for a missing key without throwing', async () => {
    const store = createMemorySheetStore();

    await expect(store.readSheet('missing')).resolves.toBeNull();
  });

  it('Edge: readSheet resolves null for a malformed record, an unsupported version and a foreign id', async () => {
    const store = createMemorySheetStore();

    store.putRawSheet(SHEET.id, 'not-an-envelope');
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    store.putRawSheet(SHEET.id, { schemaVersion: 99, id: SHEET.id, sheet: SHEET });
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    store.putRawSheet(SHEET.id, { schemaVersion: SCHEMA_VERSION, id: OTHER_ID, sheet: SHEET });
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    store.putRawSheet(SHEET.id, {
      schemaVersion: SCHEMA_VERSION,
      id: SHEET.id,
      sheet: { id: SHEET.id },
    });
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
  });

  it('Happy path: putSheet then readSheet round-trips the sheet under the current schemaVersion', async () => {
    const store = createMemorySheetStore();

    await store.putSheet(SHEET);

    await expect(store.readSheet(SHEET.id)).resolves.toEqual(SHEET);
    expect(store.records()).toEqual([
      { schemaVersion: SCHEMA_VERSION, id: SHEET.id, sheet: SHEET },
    ]);
  });

  it('Edge: deleteSheet on a missing key resolves without throwing', async () => {
    const store = createMemorySheetStore();

    await expect(store.deleteSheet('missing')).resolves.toBeUndefined();
  });
});

describe('sheet store: draft envelope (WSC-26, WSC-30, WSC-31)', () => {
  it('Edge: readDraft resolves null for a missing key, a malformed record, an unsupported version and a foreign id', async () => {
    const store = createMemorySheetStore();

    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();

    store.putRawDraft(SHEET.id, { schemaVersion: SCHEMA_VERSION, id: SHEET.id, delta: 'nope' });
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();

    store.putRawDraft(SHEET.id, {
      schemaVersion: SCHEMA_VERSION + 1,
      id: SHEET.id,
      delta: { a: 1 },
    });
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();

    store.putRawDraft(SHEET.id, { schemaVersion: SCHEMA_VERSION, id: OTHER_ID, delta: { a: 1 } });
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Happy path: writeDraft then readDraft round-trips the delta', async () => {
    const store = createMemorySheetStore();

    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });

    await expect(store.readDraft(SHEET.id)).resolves.toEqual({ stats: { dexterity: 18 } });
  });

  it('Edge: deleteDraft on a missing key resolves without throwing, and removes a present one', async () => {
    const store = createMemorySheetStore();
    await store.writeDraft(SHEET.id, { a: 1 });

    await expect(store.deleteDraft(SHEET.id)).resolves.toBeUndefined();
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
    await expect(store.deleteDraft('missing')).resolves.toBeUndefined();
  });

  it('Security: an inspection of every stored record finds no credential (WSC-31, SC-004)', async () => {
    const store = createMemorySheetStore();

    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });

    expect(credentialFree(store.records())).toBe(true);
  });
});

describe('sheet store: session clear-all (WAS-21)', () => {
  it('Happy path: the in-memory double empties both the sheets and the drafts', async () => {
    const store = createMemorySheetStore();
    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });

    await store.clearAll();

    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
    expect(store.records()).toEqual([]);
  });

  it('Edge: clearAll on an already-empty double resolves without throwing', async () => {
    const store = createMemorySheetStore();

    await expect(store.clearAll()).resolves.toBeUndefined();
  });

  it('Happy path: the idb adapter empties both object stores', async () => {
    const { store, db } = await idbHarness();
    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });

    await store.clearAll();

    await expect(db.getAll('sheets')).resolves.toEqual([]);
    await expect(db.getAll('drafts')).resolves.toEqual([]);
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Edge: the idb adapter clearAll on an already-empty database resolves without throwing', async () => {
    const { store, db } = await idbHarness();

    await expect(store.clearAll()).resolves.toBeUndefined();
    await expect(db.getAll('sheets')).resolves.toEqual([]);
    await expect(db.getAll('drafts')).resolves.toEqual([]);
  });
});

describe('sheet store: idb adapter under fake-indexeddb (WSC-11, WSC-30)', () => {
  it('Happy path: round-trips a sheet and a draft', async () => {
    const { store } = await idbHarness();

    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });

    await expect(store.readSheet(SHEET.id)).resolves.toEqual(SHEET);
    await expect(store.readDraft(SHEET.id)).resolves.toEqual({ stats: { dexterity: 18 } });
  });

  it('Edge: reads a corrupt record, an unsupported version and a foreign id as null', async () => {
    const { store, db } = await idbHarness();

    await db.put(
      'sheets',
      { schemaVersion: SCHEMA_VERSION, id: SHEET.id, sheet: 'nope' },
      SHEET.id,
    );
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    await db.put('sheets', { schemaVersion: 42, id: SHEET.id, sheet: SHEET }, SHEET.id);
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    await db.put(
      'drafts',
      { schemaVersion: SCHEMA_VERSION, id: OTHER_ID, delta: { a: 1 } },
      SHEET.id,
    );
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Edge: delete of a missing key resolves without throwing', async () => {
    const { store } = await idbHarness();

    await expect(store.deleteSheet('missing')).resolves.toBeUndefined();
    await expect(store.deleteDraft('missing')).resolves.toBeUndefined();
  });

  it('Security: the stored records carry no credential (WSC-31, SC-004)', async () => {
    const { store, db } = await idbHarness();

    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });

    const records = [...(await db.getAll('sheets')), ...(await db.getAll('drafts'))];
    expect(credentialFree(records)).toBe(true);
  });
});

describe('sheet store: cached layout and pending arrangement', () => {
  const LAYOUT = { sectionOrder: ['b', 'a'], pinned: ['a.x'] };

  it('Happy path: the cached sheet carries its committed layout', async () => {
    const store = createMemorySheetStore();
    const arranged = { ...SHEET, layout: LAYOUT };

    await store.putSheet(arranged);

    await expect(store.readSheet(SHEET.id)).resolves.toEqual(arranged);
  });

  it('Edge: a cached sheet with a malformed layout reads as null', async () => {
    const store = createMemorySheetStore();

    store.putRawSheet(SHEET.id, {
      schemaVersion: SCHEMA_VERSION,
      id: SHEET.id,
      sheet: { ...SHEET, layout: { pinned: 'x' } },
    });

    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
  });

  it.each([1, 2])(
    'Edge: a version %i record reads as missing, so old stores are rebuilt rather than migrated',
    async (schemaVersion) => {
      const store = createMemorySheetStore();

      store.putRawSheet(SHEET.id, { schemaVersion, id: SHEET.id, sheet: SHEET });
      store.putRawDraft(SHEET.id, { schemaVersion, id: SHEET.id, delta: { a: 1 } });
      store.putRawLayoutDraft(SHEET.id, { schemaVersion, id: SHEET.id, layout: LAYOUT });

      await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
      await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
      await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();
    },
  );

  it('Happy path: a pending arrangement and a staged reset round-trip', async () => {
    const store = createMemorySheetStore();

    await store.writeLayoutDraft(SHEET.id, LAYOUT);
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toEqual({ layout: LAYOUT });

    await store.writeLayoutDraft(SHEET.id, null);
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toEqual({ layout: null });
  });

  it('Edge: a missing, malformed, unsupported or foreign pending arrangement reads as null', async () => {
    const store = createMemorySheetStore();

    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();

    store.putRawLayoutDraft(SHEET.id, 'nope');
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();

    store.putRawLayoutDraft(SHEET.id, { schemaVersion: SCHEMA_VERSION, id: SHEET.id, layout: 7 });
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();

    store.putRawLayoutDraft(SHEET.id, {
      schemaVersion: SCHEMA_VERSION + 1,
      id: SHEET.id,
      layout: null,
    });
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();

    store.putRawLayoutDraft(SHEET.id, {
      schemaVersion: SCHEMA_VERSION,
      id: OTHER_ID,
      layout: null,
    });
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Edge: deleteLayoutDraft on a missing key resolves, and clearAll sweeps pending arrangements', async () => {
    const store = createMemorySheetStore();

    await expect(store.deleteLayoutDraft('missing')).resolves.toBeUndefined();
    await store.writeLayoutDraft(SHEET.id, LAYOUT);
    await store.clearAll();

    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Happy path: the idb adapter round-trips a layout draft and sweeps it with clearAll', async () => {
    const { store, db } = await idbHarness();

    await store.putSheet({ ...SHEET, layout: LAYOUT });
    await store.writeLayoutDraft(SHEET.id, LAYOUT);

    await expect(store.readSheet(SHEET.id)).resolves.toEqual({ ...SHEET, layout: LAYOUT });
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toEqual({ layout: LAYOUT });

    await store.clearAll();
    await expect(db.getAll('layoutDrafts')).resolves.toEqual([]);
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Edge: the idb adapter reads a corrupt layout draft as null and holds no credential', async () => {
    const { store, db } = await idbHarness();

    await db.put(
      'layoutDrafts',
      { schemaVersion: SCHEMA_VERSION, id: SHEET.id, layout: 'x' },
      SHEET.id,
    );
    await expect(store.readLayoutDraft(SHEET.id)).resolves.toBeNull();

    await store.writeLayoutDraft(SHEET.id, LAYOUT);
    expect(credentialFree(await db.getAll('layoutDrafts'))).toBe(true);
    await expect(store.deleteLayoutDraft('missing')).resolves.toBeUndefined();
  });
});

describe('sheet store: lazy wrapper', () => {
  const LAYOUT = { sectionOrder: [], pinned: ['a.x'] };

  it('Happy path: forwards every operation, the pending arrangement included, to the opened store', async () => {
    const lazy = createLazySheetStore(() => Promise.resolve(createMemorySheetStore()));

    await lazy.putSheet(SHEET);
    await lazy.writeDraft(SHEET.id, { a: 1 });
    await lazy.writeLayoutDraft(SHEET.id, LAYOUT);

    await expect(lazy.readSheet(SHEET.id)).resolves.toEqual(SHEET);
    await expect(lazy.readDraft(SHEET.id)).resolves.toEqual({ a: 1 });
    await expect(lazy.readLayoutDraft(SHEET.id)).resolves.toEqual({ layout: LAYOUT });

    await lazy.deleteLayoutDraft(SHEET.id);
    await lazy.deleteDraft(SHEET.id);
    await lazy.deleteSheet(SHEET.id);
    await expect(lazy.readLayoutDraft(SHEET.id)).resolves.toBeNull();
    await expect(lazy.readDraft(SHEET.id)).resolves.toBeNull();
    await expect(lazy.readSheet(SHEET.id)).resolves.toBeNull();
    await lazy.writeLayoutDraft(SHEET.id, null);
    await lazy.clearAll();
    await expect(lazy.readLayoutDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Edge: a failed opening swallows deletes and clearAll, rejects reads and writes, and retries the opening', async () => {
    let attempts = 0;
    const lazy = createLazySheetStore(() => {
      attempts += 1;
      return attempts === 1
        ? Promise.reject(new Error('idb'))
        : Promise.resolve(createMemorySheetStore());
    });

    await expect(lazy.deleteLayoutDraft(SHEET.id)).resolves.toBeUndefined();
    await expect(lazy.readLayoutDraft(SHEET.id)).resolves.toBeNull();
    expect(attempts).toBe(2);

    const failing = createLazySheetStore(() => Promise.reject(new Error('idb')));
    await expect(failing.readLayoutDraft(SHEET.id)).rejects.toThrow('idb');
    await expect(failing.writeLayoutDraft(SHEET.id, null)).rejects.toThrow('idb');
    await expect(failing.clearAll()).resolves.toBeUndefined();
  });
});
