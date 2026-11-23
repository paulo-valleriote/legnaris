import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { parseSheet, valueAtPath } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { RECORD_ID_PATTERN, orderedRecordIDs } from '../engine/row_identity';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import {
  createSheetEditStore,
  isPending,
  pendingCount,
  type SheetEditStore,
} from '../sheet/sheet_edit';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import {
  LIST_PATH,
  LIST_RECORD_A,
  LIST_RECORD_B,
  LIST_RECORD_C,
  listDataWithRoom,
  listSheetPayload,
  listTemplatePayload,
} from './list_fixture';
import { SHEET_ID } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

function clientWith(transport: Parameters<typeof createApiClient>[0]['transport']) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function storeWith(
  transport: Parameters<typeof createApiClient>[0]['transport'] = createFakeTransport(
    successOutcome({}),
  ).transport,
  store = createMemorySheetStore(),
  data?: Record<string, unknown>,
): SheetEditStore {
  return createSheetEditStore(
    {
      template: parseTemplateDetail(listTemplatePayload()),
      sheet: parseSheet(listSheetPayload(data === undefined ? {} : { data })),
    },
    {
      client: clientWith(transport),
      store,
      saveRetry: { sleep: () => Promise.resolve(), random: () => 0 },
    },
  );
}

function ids(store: SheetEditStore): readonly string[] {
  return orderedRecordIDs(
    parseTemplateDetail(listTemplatePayload()).fields,
    LIST_PATH,
    store.getState().data,
  );
}

function record(store: SheetEditStore, id: string): Record<string, unknown> | null {
  const items = valueAtPath(store.getState().data, LIST_PATH);
  return (items as Record<string, Record<string, unknown> | null>)[id] ?? null;
}

describe('record lists in the edit store', () => {
  it('Happy path: adding seeds the editable shape and appends a numeric position', () => {
    const store = storeWith(undefined, undefined, listDataWithRoom());

    const id = store.addRecord(LIST_PATH);

    expect(id).toBeDefined();
    expect(RECORD_ID_PATTERN.test(id ?? '')).toBe(true);
    expect(ids(store)).toEqual([LIST_RECORD_A, id]);
    expect(record(store, id ?? '')).toMatchObject({
      name: '',
      quantity: 1,
      weight: 1,
      active: 0,
      optionA: 0,
      optionB: 0,
      position: 2,
    });
    expect(record(store, id ?? '')).not.toHaveProperty('derivedTotal');
    expect(store.getPendingDelta()).toMatchObject({
      inventory: { items: { [id ?? '']: { name: '', quantity: 1, position: 2 } } },
    });
    expect(isPending(store.getState())).toBe(true);
  });

  it('Edge: adding at capacity returns nothing and leaves the sheet untouched', () => {
    const store = storeWith();
    const before = store.getState();

    expect(store.addRecord(LIST_PATH)).toBeUndefined();
    expect(store.getState().data).toBe(before.data);
    expect(store.getState().revision).toBe(before.revision);
    expect(isPending(store.getState())).toBe(false);
  });

  it('Edge: adding to a list the template does not declare returns nothing', () => {
    const store = storeWith();

    expect(store.addRecord('inventory.missing')).toBeUndefined();
    expect(isPending(store.getState())).toBe(false);
  });

  it('Happy path: removing a record sends a null under its key and hides it from the list', () => {
    const store = storeWith();

    store.removeRecord(LIST_PATH, LIST_RECORD_A);

    expect(ids(store)).toEqual([LIST_RECORD_B]);
    expect(store.getPendingDelta()).toEqual({
      inventory: { items: { [LIST_RECORD_A]: null } },
    });
    expect(isPending(store.getState())).toBe(true);
  });

  it('Edge: removing an absent record is a no-op, and a second removal changes nothing', () => {
    const store = storeWith();
    store.removeRecord(LIST_PATH, LIST_RECORD_A);
    const once = store.getState();

    store.removeRecord(LIST_PATH, LIST_RECORD_A);
    store.removeRecord(LIST_PATH, LIST_RECORD_C);

    expect(store.getState().data).toBe(once.data);
    expect(store.getState().revision).toBe(once.revision);
    expect(store.getPendingDelta()).toEqual({
      inventory: { items: { [LIST_RECORD_A]: null } },
    });
  });

  it('Edge: adding then removing the same unsaved record leaves nothing pending', () => {
    const store = storeWith(undefined, undefined, listDataWithRoom());
    const id = store.addRecord(LIST_PATH);

    store.removeRecord(LIST_PATH, id ?? '');

    expect(ids(store)).toEqual([LIST_RECORD_A]);
    expect(store.getPendingDelta()).toEqual({});
    expect(isPending(store.getState())).toBe(false);
  });

  it('Happy path: moving down rewrites the two positions and the list order', () => {
    const store = storeWith();

    expect(store.moveRecord(LIST_PATH, LIST_RECORD_A, 1)).toBe(2);

    expect(ids(store)).toEqual([LIST_RECORD_B, LIST_RECORD_A]);
    expect(store.getPendingDelta()).toEqual({
      inventory: {
        items: {
          [LIST_RECORD_A]: { position: 2 },
          [LIST_RECORD_B]: { position: 1 },
        },
      },
    });
  });

  it('Edge: moving past either end is a no-op', () => {
    const store = storeWith();
    const before = store.getState();

    expect(store.moveRecord(LIST_PATH, LIST_RECORD_A, -1)).toBeUndefined();
    expect(store.moveRecord(LIST_PATH, LIST_RECORD_B, 1)).toBeUndefined();
    expect(store.moveRecord(LIST_PATH, LIST_RECORD_C, 1)).toBeUndefined();

    expect(store.getState().data).toBe(before.data);
    expect(store.getState().revision).toBe(before.revision);
    expect(isPending(store.getState())).toBe(false);
  });

  it('Happy path: discard restores a removed record and drops an added one', () => {
    const store = storeWith();
    store.removeRecord(LIST_PATH, LIST_RECORD_A);
    const added = store.addRecord(LIST_PATH);

    store.discard();

    expect(added).toBeDefined();
    expect(ids(store)).toEqual([LIST_RECORD_A, LIST_RECORD_B]);
    expect(isPending(store.getState())).toBe(false);
  });

  it('Happy path: a restored draft brings back an added record and a removal', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, {
      inventory: {
        items: {
          [LIST_RECORD_A]: null,
          [LIST_RECORD_C]: { position: 3, name: 'Bow', quantity: 4 },
        },
      },
    });
    const store = storeWith(undefined, sheetStore);

    await store.restore();

    expect(ids(store)).toEqual([LIST_RECORD_B, LIST_RECORD_C]);
    expect(record(store, LIST_RECORD_C)).toMatchObject({ name: 'Bow', quantity: 4 });
    expect(isPending(store.getState())).toBe(true);
  });

  it('Edge: a restored draft cannot write a derived field or an outside path', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, {
      ghost: 9,
      inventory: {
        items: {
          [LIST_RECORD_C]: { position: 3, name: 'Bow', derivedTotal: 99 },
          'not-a-record': { position: 4, name: 'Ghost' },
        },
      },
    });
    const store = storeWith(undefined, sheetStore);

    await store.restore();

    expect(record(store, LIST_RECORD_C)).not.toHaveProperty('derivedTotal');
    expect(valueAtPath(store.getState().data, LIST_PATH)).not.toHaveProperty('not-a-record');
    expect(valueAtPath(store.getState().data, 'ghost')).toBeNull();
  });

  it('Edge: a 409 on save keeps the added record and its pending delta', async () => {
    const fake = createFakeTransport(errorOutcome('CONFLICT', 'stale', 409));
    const store = storeWith(fake.transport, undefined, listDataWithRoom());
    const id = store.addRecord(LIST_PATH);

    await store.save();

    expect(fake.attempts).toBe(3);
    expect(store.getState().saveError).toBe(message('sheet.error.saveConflict'));
    expect(record(store, id ?? '')).not.toBeNull();
    expect(isPending(store.getState())).toBe(true);
    expect(pendingCount(store.getState())).toBeGreaterThan(0);
  });

  it('Happy path: a saved record advances the baseline and clears the pending work', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const store = storeWith(fake.transport, undefined, listDataWithRoom());
    const id = store.addRecord(LIST_PATH);

    await store.save();

    expect(fake.lastRequest?.method).toBe('PATCH');
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toMatchObject({
      data: { inventory: { items: { [id ?? '']: expect.objectContaining({ position: 2 }) } } },
    });
    expect(record(store, id ?? '')).not.toBeNull();
    expect(isPending(store.getState())).toBe(false);
    expect(store.getPendingDelta()).toEqual({});
  });
});
