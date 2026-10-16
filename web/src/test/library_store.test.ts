import 'fake-indexeddb/auto';
import { describe, expect, it } from 'vitest';
import type { IDBPDatabase } from 'idb';
import { wrap } from 'idb';
import { IDBFactory as FDBFactory } from 'fake-indexeddb';
import type { Sheet } from '../api/models/sheet';
import { createIdbSheetStore, SHEET_DATABASE_NAME } from '../library/idb_sheet_store';
import type { SheetStore } from '../library/sheet_store';
import { SCHEMA_VERSION, createMemorySheetStore } from '../library/sheet_store';

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

    store.putRawSheet(SHEET.id, { schemaVersion: 1, id: OTHER_ID, sheet: SHEET });
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    store.putRawSheet(SHEET.id, { schemaVersion: 1, id: SHEET.id, sheet: { id: SHEET.id } });
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
  });

  it('Happy path: putSheet then readSheet round-trips the sheet under schemaVersion 1', async () => {
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

    store.putRawDraft(SHEET.id, { schemaVersion: 1, id: SHEET.id, delta: 'nope' });
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();

    store.putRawDraft(SHEET.id, { schemaVersion: 2, id: SHEET.id, delta: { a: 1 } });
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();

    store.putRawDraft(SHEET.id, { schemaVersion: 1, id: OTHER_ID, delta: { a: 1 } });
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

    await db.put('sheets', { schemaVersion: 1, id: SHEET.id, sheet: 'nope' }, SHEET.id);
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    await db.put('sheets', { schemaVersion: 42, id: SHEET.id, sheet: SHEET }, SHEET.id);
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();

    await db.put('drafts', { schemaVersion: 1, id: OTHER_ID, delta: { a: 1 } }, SHEET.id);
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
