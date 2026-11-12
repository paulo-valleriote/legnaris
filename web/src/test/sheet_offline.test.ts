import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from '../api/client';
import { failureMessage } from '../api/error_mapper';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import { createSheetEditStore, isPending } from '../sheet/sheet_edit';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { sheetPayload, templateDetailPayload } from './sheet_fixtures';

function storeWith(outcomes: Parameters<typeof createFakeTransport>) {
  const fake = createFakeTransport(...outcomes);
  const client = createApiClient({ baseUrl: 'http://api.test', transport: fake.transport });
  const sheetStore = createMemorySheetStore();
  const sheet = parseSheet(sheetPayload());
  const store = createSheetEditStore(
    { template: parseTemplateDetail(templateDetailPayload()), sheet },
    { client, store: sheetStore },
  );
  return { store, sheetStore, sheetId: sheet.id };
}

const OFFLINE = { failure: new Error('down') };

describe('offline save and preview keep the work on the device', () => {
  it('Edge: a draft write that fails says the changes were not kept instead of promising', async () => {
    const { store, sheetStore } = storeWith([OFFLINE]);
    vi.spyOn(sheetStore, 'writeDraft').mockRejectedValue(new Error('quota'));

    store.setPath('identity.level', 4);
    await store.save();

    expect(store.getState().saveError).toBe(message('sheet.error.saveOfflineUnkept'));
    expect(store.getState().saveErrorKind).toBe('offline');
    expect(isPending(store.getState())).toBe(true);
  });

  it('Edge: a failed preview offline with the draft write failing does not promise either', async () => {
    const { store, sheetStore } = storeWith([OFFLINE]);
    vi.spyOn(sheetStore, 'writeDraft').mockRejectedValue(new Error('private mode'));

    store.setPath('identity.level', 4);
    await store.preview();

    expect(store.getState().previewError).toBe(message('sheet.error.previewOfflineUnkept'));
  });

  it('Edge: a non-offline save failure writes no draft and keeps the mapped copy and the normal tone', async () => {
    const { store, sheetStore, sheetId } = storeWith([errorOutcome('INTERNAL_ERROR', 'boom', 500)]);

    store.setPath('identity.level', 4);
    await store.save();

    expect(store.getState().saveError).toBe(failureMessage('serverError'));
    expect(store.getState().saveErrorKind).toBe('serverError');
    await expect(sheetStore.readDraft(sheetId)).resolves.toBeNull();
  });

  it('Edge: a successful retry clears the offline copy and its kind', async () => {
    const { store } = storeWith([OFFLINE, successOutcome({})]);

    store.setPath('identity.level', 4);
    await store.save();
    await store.save();

    expect(store.getState().saveError).toBeUndefined();
    expect(store.getState().saveErrorKind).toBeUndefined();
  });

  it('Happy path: an offline save writes the pending delta to the draft at once and says so', async () => {
    const { store, sheetStore, sheetId } = storeWith([OFFLINE]);

    store.setPath('identity.level', 4);
    await store.save();

    expect(store.getState().saveError).toBe(message('sheet.error.saveOffline'));
    expect(store.getState().saveErrorKind).toBe('offline');
    await expect(sheetStore.readDraft(sheetId)).resolves.toEqual({
      delta: { identity: { level: 4 } },
    });
  });

  it('Happy path: an offline preview writes the draft and offers to preview again', async () => {
    const { store, sheetStore, sheetId } = storeWith([OFFLINE]);

    store.setPath('identity.level', 4);
    await store.preview();

    expect(store.getState().previewError).toBe(message('sheet.error.previewOffline'));
    await expect(sheetStore.readDraft(sheetId)).resolves.toEqual({
      delta: { identity: { level: 4 } },
    });
  });
});
