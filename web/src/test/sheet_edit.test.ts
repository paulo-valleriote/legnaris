import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from '../api/client';
import { failureMessage } from '../api/error_mapper';
import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import type { SaveRetryDeps } from '../sheet/save_retry';
import { canSave, createSheetEditStore, isPending, type SheetEditStore } from '../sheet/sheet_edit';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

const ABILITY_PATHS = ['ability.stealth', 'ability.athletics'] as const;

function template() {
  return parseTemplateDetail(templateDetailPayload());
}

function sheet(overrides: Record<string, unknown> = {}): Sheet {
  return parseSheet(sheetPayload(overrides));
}

function storeWith(
  transport: Parameters<typeof createApiClient>[0]['transport'],
  data?: Record<string, unknown>,
  saveRetry?: SaveRetryDeps,
) {
  const client = createApiClient({ baseUrl: BASE_URL, transport });
  const sheetStore = createMemorySheetStore();
  const store = createSheetEditStore(
    { template: template(), sheet: sheet(data === undefined ? {} : { data }) },
    { client, store: sheetStore, ...(saveRetry === undefined ? {} : { saveRetry }) },
  );
  return { store, sheetStore };
}

function dataOf(store: SheetEditStore): Record<string, unknown> {
  return store.getState().data;
}

describe('sheet edit store (WSC-12 … WSC-19, WSC-34)', () => {
  it('Edge: a redundant write advances no revision (WSC-12)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setPath('identity.level', 4);
    const revision = store.getState().revision;

    store.setPath('identity.level', 4);

    expect(store.getState().revision).toBe(revision);
  });

  it('Edge: an edit back to its seeded value un-pends and leaves the delta empty (WSC-13)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setPath('identity.level', 4);
    store.setPath('identity.level', 3);

    expect(isPending(store.getState())).toBe(false);
    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a derived (non-editable) path never enters the delta (WSC-14)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setPath('derived.total', 99);

    expect(store.getPendingDelta()).toEqual({});
    expect(isPending(store.getState())).toBe(false);
    expect(valueAtPath(dataOf(store), 'derived.total')).toBe(4);
  });

  it('Edge: an unchanged sibling of a changed nested group never enters the delta (WSC-14)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setPath('identity.characterName', 'Boromir');

    expect(store.getPendingDelta()).toEqual({ identity: { characterName: 'Boromir' } });
  });

  it('Edge: a high-precision decimal is sent unchanged (WSC-14)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setPath('identity.level', 3.141592653589793);

    expect(store.getPendingDelta()).toEqual({ identity: { level: 3.141592653589793 } });
  });

  it('Edge: an exclusive selection writes one member on and its siblings off in one emission (WSC-15)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport, {
      identity: { characterName: 'Aragorn', level: 3 },
      derived: { total: 4 },
      ability: { athletics: 1 },
    });
    const emissions = vi.fn();
    store.subscribe(emissions);

    store.setExclusive('ability.stealth', ABILITY_PATHS);

    expect(valueAtPath(dataOf(store), 'ability.stealth')).toBe(1);
    expect(valueAtPath(dataOf(store), 'ability.athletics')).toBe(0);
    expect(store.getPendingDelta()).toEqual({ ability: { stealth: 1, athletics: 0 } });
    expect(emissions).toHaveBeenCalledTimes(1);
  });

  it('Edge: selecting None writes every member off (WSC-15)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setExclusive('ability.stealth', ABILITY_PATHS);
    store.setExclusive(null, ABILITY_PATHS);

    expect(valueAtPath(dataOf(store), 'ability.stealth')).toBe(0);
    expect(valueAtPath(dataOf(store), 'ability.athletics')).toBe(0);
    expect(isPending(store.getState())).toBe(false);
    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a redundant exclusive write emits nothing (WSC-15)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);

    store.setExclusive('ability.stealth', ABILITY_PATHS);
    const revision = store.getState().revision;

    store.setExclusive('ability.stealth', ABILITY_PATHS);

    expect(store.getState().revision).toBe(revision);
  });

  it('Edge: subscribePath notifies only the listeners of the changed path (WSC-34)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);
    const changed = vi.fn();
    const untouched = vi.fn();
    store.subscribePath('identity.level', changed);
    store.subscribePath('identity.characterName', untouched);

    store.setPath('identity.level', 4);

    expect(changed).toHaveBeenCalledTimes(1);
    expect(untouched).not.toHaveBeenCalled();
  });

  it('Edge: an unsubscribed path listener is not notified (WSC-34)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);
    const listener = vi.fn();
    const unsubscribe = store.subscribePath('identity.level', listener);

    unsubscribe();
    store.setPath('identity.level', 4);

    expect(listener).not.toHaveBeenCalled();
  });

  it('Edge: Save with no pending edit issues no request (WSC-19)', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const { store } = storeWith(fake.transport);

    expect(canSave(store.getState())).toBe(false);
    await store.save();

    expect(fake.attempts).toBe(0);
  });

  it('Edge: a second Save while one is in flight issues no request (WSC-18)', async () => {
    const deferred = createDeferredTransport();
    const { store } = storeWith(deferred.transport);

    store.setPath('identity.level', 4);
    const first = store.save();
    const second = store.save();

    expect(deferred.attempts).toBe(1);

    deferred.resolve(successOutcome({}));
    await Promise.all([first, second]);

    expect(deferred.attempts).toBe(1);
  });

  it('Edge: a failed save keeps every edit, shows the mapped message and issues no retry (WSC-17)', async () => {
    const fake = createFakeTransport(errorOutcome('INTERNAL_ERROR', 'boom', 500));
    const { store } = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    await store.save();

    expect(store.getState().saveError).toBe(failureMessage('serverError'));
    expect(isPending(store.getState())).toBe(true);
    expect(store.getPendingDelta()).toEqual({ identity: { level: 4 } });
    expect(fake.attempts).toBe(1);
  });

  it('Happy path: a 409 on save is resent after the Retry-After wait and the second send wins', async () => {
    const sleep = vi.fn().mockResolvedValue(undefined);
    const fake = createFakeTransport(
      { ...errorOutcome('CONFLICT', 'busy', 409), retryAfter: '1' },
      successOutcome({}),
    );
    const { store } = storeWith(fake.transport, undefined, { sleep, random: () => 0 });

    store.setPath('identity.level', 4);
    await store.save();

    expect(fake.attempts).toBe(2);
    expect(sleep).toHaveBeenCalledExactlyOnceWith(1000);
    expect(store.getState().saveError).toBeUndefined();
    expect(isPending(store.getState())).toBe(false);
    expect(fake.requests[1]?.body).toBe(fake.requests[0]?.body);
  });

  it('Error: a persistent 409 stops after three sends and shows the conflict copy, keeping the edits', async () => {
    const sleep = vi.fn().mockResolvedValue(undefined);
    const fake = createFakeTransport(errorOutcome('CONFLICT', 'busy', 409));
    const { store } = storeWith(fake.transport, undefined, { sleep, random: () => 0 });

    store.setPath('identity.level', 4);
    await store.save();

    expect(fake.attempts).toBe(3);
    expect(store.getState().saveError).toBe(message('sheet.error.saveConflict'));
    expect(store.getState().saveErrorKind).toBe('conflict');
    expect(store.getState().isSaving).toBe(false);
    expect(store.getPendingDelta()).toEqual({ identity: { level: 4 } });
  });

  it('Edge: a non-409 failure is not resent and keeps its own copy', async () => {
    const sleep = vi.fn().mockResolvedValue(undefined);
    const fake = createFakeTransport(errorOutcome('RATE_LIMITED', 'slow down', 429));
    const { store } = storeWith(fake.transport, undefined, { sleep, random: () => 0 });

    store.setPath('identity.level', 4);
    await store.save();

    expect(fake.attempts).toBe(1);
    expect(sleep).not.toHaveBeenCalled();
    expect(store.getState().saveError).toBe(failureMessage('rateLimited'));
  });

  it('Edge: an edit made during a save stays pending after it succeeds (WSC-16)', async () => {
    const deferred = createDeferredTransport();
    const { store } = storeWith(deferred.transport);

    store.setPath('identity.level', 4);
    const saving = store.save();
    store.setPath('identity.characterName', 'Boromir');

    deferred.resolve(successOutcome({ derived: { total: 5 } }));
    await saving;

    const state = store.getState();
    expect(isPending(state)).toBe(true);
    expect(store.getPendingDelta()).toEqual({ identity: { characterName: 'Boromir' } });
    expect(valueAtPath(state.data, 'identity.level')).toBe(4);
    expect(valueAtPath(state.defaults, 'identity.level')).toBe(4);
    expect(valueAtPath(state.data, 'identity.characterName')).toBe('Boromir');
    expect(valueAtPath(state.data, 'derived.total')).toBe(5);
  });

  it('Happy path: setPath records the path and produces a fresh data tree (WSC-12)', () => {
    const { store } = storeWith(createFakeTransport(successOutcome({})).transport);
    const before = dataOf(store);

    store.setPath('identity.level', 4);

    const after = dataOf(store);
    expect(after).not.toBe(before);
    expect(after['identity']).not.toBe(before['identity']);
    expect(valueAtPath(before, 'identity.level')).toBe(3);
    expect(valueAtPath(after, 'identity.level')).toBe(4);
    expect(store.getState().editedPaths.has('identity.level')).toBe(true);
    expect(store.getPendingDelta()).toEqual({ identity: { level: 4 } });
  });

  it('Happy path: save sends the exact nested delta with one PATCH (WSC-14)', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const { store } = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    store.setPath('identity.characterName', 'Boromir');
    await store.save();

    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('PATCH');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/sheets/${SHEET_ID}`);
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      data: { identity: { level: 4, characterName: 'Boromir' } },
    });
  });

  it('Happy path: a successful save merges the cascade and advances the baseline (WSC-16)', async () => {
    const fake = createFakeTransport(successOutcome({ derived: { total: 9 } }));
    const { store } = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    await store.save();

    const state = store.getState();
    expect(isPending(state)).toBe(false);
    expect(state.saveError).toBeUndefined();
    expect(valueAtPath(state.data, 'derived.total')).toBe(9);
    expect(valueAtPath(state.defaults, 'derived.total')).toBe(9);
    expect(valueAtPath(state.defaults, 'identity.level')).toBe(4);
  });

  it('Happy path: a successful save deletes the sheet draft (WSC-29)', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const { store, sheetStore } = storeWith(fake.transport);
    await sheetStore.writeDraft(SHEET_ID, { identity: { level: 9 } });

    store.setPath('identity.level', 4);
    await store.save();

    expect(await sheetStore.readDraft(SHEET_ID)).toBeNull();
  });
});
