import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import type { Sheet } from '../api/models/sheet';
import { parseSheet, valueAtPath } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { DRAFT_CADENCE_MS, startDraftCadence } from '../library/draft_backup';
import { SCHEMA_VERSION, createMemorySheetStore } from '../library/sheet_store';
import { createSheetEditStore, isPending } from '../sheet/sheet_edit';
import { createFakeTransport, successOutcome } from './fake_transport';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

interface ManualScheduler {
  readonly scheduler: (tick: () => void, intervalMs: number) => () => void;
  readonly intervalMs: () => number | undefined;
  tick(): void;
  stopped(): boolean;
}

/** A hand-driven ticker so the cadence is exercised without waiting a real minute (R6). */
function manualScheduler(): ManualScheduler {
  let callback: (() => void) | undefined;
  let interval: number | undefined;
  let cancelled = false;

  return {
    scheduler: (tick, intervalMs) => {
      callback = tick;
      interval = intervalMs;
      return () => {
        cancelled = true;
        callback = undefined;
      };
    },
    intervalMs: () => interval,
    tick: () => callback?.(),
    stopped: () => cancelled,
  };
}

function sheet(overrides: Record<string, unknown> = {}): Sheet {
  return parseSheet(sheetPayload(overrides));
}

function editHarness(sheetStore = createMemorySheetStore()) {
  const client = createApiClient({
    baseUrl: BASE_URL,
    transport: createFakeTransport(successOutcome({})).transport,
  });
  const store = createSheetEditStore(
    { template: parseTemplateDetail(templateDetailPayload()), sheet: sheet() },
    { client, store: sheetStore },
  );
  return { store, sheetStore };
}

function pendingSnapshot(store: ReturnType<typeof editHarness>['store']) {
  return isPending(store.getState()) ? { sheetId: SHEET_ID, delta: store.getPendingDelta() } : null;
}

describe('draft cadence (WSC-26, WSC-27, R6)', () => {
  it('Edge: a clean sheet writes nothing (WSC-27)', () => {
    const sheetStore = createMemorySheetStore();
    const scheduler = manualScheduler();
    startDraftCadence(sheetStore, () => null, scheduler.scheduler);

    scheduler.tick();

    expect(sheetStore.records()).toHaveLength(0);
  });

  it('Edge: a tick with an empty delta writes nothing (WSC-27)', () => {
    const sheetStore = createMemorySheetStore();
    const scheduler = manualScheduler();
    startDraftCadence(sheetStore, () => ({ sheetId: SHEET_ID, delta: {} }), scheduler.scheduler);

    scheduler.tick();

    expect(sheetStore.records()).toHaveLength(0);
  });

  it('Happy path: a pending tick writes the current delta at the 60-second cadence (WSC-26)', () => {
    const sheetStore = createMemorySheetStore();
    const scheduler = manualScheduler();
    startDraftCadence(
      sheetStore,
      () => ({ sheetId: SHEET_ID, delta: { identity: { level: 4 } } }),
      scheduler.scheduler,
    );

    scheduler.tick();

    expect(scheduler.intervalMs()).toBe(DRAFT_CADENCE_MS);
    expect(sheetStore.records()).toEqual([
      { schemaVersion: SCHEMA_VERSION, id: SHEET_ID, delta: { identity: { level: 4 } } },
    ]);
  });

  it('Edge: a write failure is swallowed and the cadence keeps running (WSC-26)', async () => {
    const sheetStore = createMemorySheetStore();
    const attempts: Array<Record<string, unknown>> = [];
    const failing = {
      ...sheetStore,
      writeDraft: (_id: string, delta: Record<string, unknown>) => {
        attempts.push(delta);
        return Promise.reject(new Error('quota exceeded'));
      },
    };
    const scheduler = manualScheduler();
    startDraftCadence(
      failing,
      () => ({ sheetId: SHEET_ID, delta: { identity: { level: 4 } } }),
      scheduler.scheduler,
    );

    expect(() => scheduler.tick()).not.toThrow();
    scheduler.tick();
    await Promise.resolve();

    expect(attempts).toHaveLength(2);
  });

  it('Edge: stopping the cadence stops the writes (WSC-26)', () => {
    const sheetStore = createMemorySheetStore();
    const scheduler = manualScheduler();
    const stop = startDraftCadence(
      sheetStore,
      () => ({ sheetId: SHEET_ID, delta: { identity: { level: 4 } } }),
      scheduler.scheduler,
    );

    stop();

    expect(scheduler.stopped()).toBe(true);
    scheduler.tick();
    expect(sheetStore.records()).toHaveLength(0);
  });

  it('Happy path: a tick fed by the edit store writes only the editable delta (WSC-26)', () => {
    const { store, sheetStore } = editHarness();
    store.setPath('identity.level', 4);
    store.setPath('derived.total', 99);
    const scheduler = manualScheduler();
    startDraftCadence(sheetStore, () => pendingSnapshot(store), scheduler.scheduler);

    scheduler.tick();

    expect(sheetStore.records()).toEqual([
      { schemaVersion: SCHEMA_VERSION, id: SHEET_ID, delta: { identity: { level: 4 } } },
    ]);
  });

  it('Security: a cadence-written draft carries no credential (WSC-31, SC-004)', async () => {
    const { store, sheetStore } = editHarness();
    await sheetStore.putSheet(sheet());
    store.setPath('identity.level', 4);
    const scheduler = manualScheduler();
    startDraftCadence(sheetStore, () => pendingSnapshot(store), scheduler.scheduler);

    scheduler.tick();

    expect(JSON.stringify(sheetStore.records())).not.toMatch(
      /token|password|secret|authorization|cookie|credential/i,
    );
  });
});

describe('draft restore and lifecycle (WSC-28, WSC-29, WSC-30)', () => {
  it('Edge: a corrupt or unsupported-version draft seeds the sheet cleanly (WSC-30)', async () => {
    const sheetStore = createMemorySheetStore();
    sheetStore.putRawDraft(SHEET_ID, 'not-an-envelope');
    const { store } = editHarness(sheetStore);

    await expect(store.restore()).resolves.toBeUndefined();
    expect(isPending(store.getState())).toBe(false);

    sheetStore.putRawDraft(SHEET_ID, {
      schemaVersion: SCHEMA_VERSION + 8,
      id: SHEET_ID,
      delta: { identity: { level: 9 } },
    });
    await store.restore();

    expect(isPending(store.getState())).toBe(false);
    expect(valueAtPath(store.getState().data, 'identity.level')).toBe(3);
  });

  it('Edge: a draft key that is not an editable leaf is ignored on restore (WSC-28)', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, { derived: { total: 99 }, identity: { level: 5 } });
    const { store } = editHarness(sheetStore);

    await store.restore();

    const state = store.getState();
    expect(valueAtPath(state.data, 'identity.level')).toBe(5);
    expect(valueAtPath(state.data, 'derived.total')).toBe(4);
    expect(state.editedPaths.has('identity.level')).toBe(true);
    expect(state.editedPaths.has('derived.total')).toBe(false);
    expect(store.getPendingDelta()).toEqual({ identity: { level: 5 } });
  });

  it('Happy path: reopening overlays only the editable keys and marks them pending (WSC-28)', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, {
      identity: { level: 8, characterName: 'Boromir' },
    });
    const { store } = editHarness(sheetStore);

    await store.restore();

    const state = store.getState();
    expect(isPending(state)).toBe(true);
    expect(valueAtPath(state.data, 'identity.level')).toBe(8);
    expect(valueAtPath(state.data, 'identity.characterName')).toBe('Boromir');
    expect(valueAtPath(state.defaults, 'identity.level')).toBe(3);
    expect(state.previewMarks).toBeUndefined();
  });

  it('Happy path: saving a restored draft deletes it and advances the baseline (WSC-29)', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, { identity: { level: 8 } });
    const client = createApiClient({
      baseUrl: BASE_URL,
      transport: createFakeTransport(successOutcome({ derived: { total: 9 } })).transport,
    });
    const store = createSheetEditStore(
      { template: parseTemplateDetail(templateDetailPayload()), sheet: sheet() },
      { client, store: sheetStore },
    );
    await store.restore();

    await store.save();

    expect(isPending(store.getState())).toBe(false);
    expect(valueAtPath(store.getState().data, 'derived.total')).toBe(9);
    expect(await sheetStore.readDraft(SHEET_ID)).toBeNull();
  });
});
