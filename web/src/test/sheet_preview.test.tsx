import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { failureMessage } from '../api/error_mapper';
import type { Sheet } from '../api/models/sheet';
import { parseSheet, valueAtPath } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { FIELD_MARKED_CLASS } from '../engine/sheet_mark';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import { canPreview, createSheetEditStore, isPending } from '../sheet/sheet_edit';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${SHEET_ID}`;

function clientWith(transport: Parameters<typeof createApiClient>[0]['transport']) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function storeWith(transport: Parameters<typeof createApiClient>[0]['transport']) {
  const sheet: Sheet = parseSheet(sheetPayload());
  return createSheetEditStore(
    { template: parseTemplateDetail(templateDetailPayload()), sheet },
    { client: clientWith(transport), store: createMemorySheetStore() },
  );
}

function previewRequests(fake: ReturnType<typeof createFakeTransport>) {
  return fake.requests.filter((request) => request.url.endsWith('/preview'));
}

describe('sheet preview store (WSC-21 … WSC-25)', () => {
  it('Edge: Preview with an empty delta issues no request (WSC-21)', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const store = storeWith(fake.transport);

    await store.preview();

    expect(fake.attempts).toBe(0);
    expect(store.getState().previewError).toBeUndefined();
  });

  it('Edge: a repeat preview with no new edit issues no request (WSC-21, WSC-24)', async () => {
    const fake = createFakeTransport(successOutcome({ derived: { total: 5 } }));
    const store = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    await store.preview();
    await store.preview();

    expect(previewRequests(fake)).toHaveLength(1);
    expect(canPreview(store.getState())).toBe(false);
  });

  it('Edge: a failed preview keeps every edit, shows the mapped message and stays available (WSC-25)', async () => {
    const fake = createFakeTransport(errorOutcome('INTERNAL_ERROR', 'boom', 500));
    const store = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    await store.preview();

    expect(store.getState().previewError).toBe(failureMessage('serverError'));
    expect(isPending(store.getState())).toBe(true);
    expect(canPreview(store.getState())).toBe(true);
  });

  it('Edge: an edit after a preview clears the marks and re-enables Preview (WSC-24)', async () => {
    const fake = createFakeTransport(successOutcome({ derived: { total: 5 } }));
    const store = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    await store.preview();
    expect(store.getState().previewMarks).toBeDefined();

    store.setPath('identity.level', 5);

    expect(store.getState().previewMarks).toBeUndefined();
    expect(canPreview(store.getState())).toBe(true);
  });

  it('Happy path: Preview sends the delta, merges the cascade without advancing the baseline and publishes the marks (WSC-21, WSC-22, WSC-23)', async () => {
    const fake = createFakeTransport(successOutcome({ derived: { total: 7 } }));
    const store = storeWith(fake.transport);

    store.setPath('identity.level', 4);
    await store.preview();

    const state = store.getState();
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/sheets/${SHEET_ID}/preview`);
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      data: { identity: { level: 4 } },
    });
    expect(valueAtPath(state.data, 'derived.total')).toBe(7);
    expect(isPending(state)).toBe(true);
    expect(store.getPendingDelta()).toEqual({ identity: { level: 4 } });
    expect(state.previewMarks?.modified).toEqual(new Set(['identity.level']));
    expect(state.previewMarks?.recalculated).toEqual(new Set(['derived.total']));
  });
});

describe('sheet preview marks on the screen (WSC-21, WSC-23, SC-009)', () => {
  it('Happy path: Preview tints and labels the changed and recalculated rows through the engine seam (WSC-23, SC-009)', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome({ derived: { total: 5 } }),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));

    await waitFor(() => expect(previewRequests(fake)).toHaveLength(1));

    expect(await screen.findByText(message('engine.modified'))).toBeInTheDocument();
    expect(screen.getByText(message('engine.recalculated'))).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveClass(FIELD_MARKED_CLASS);
    expect(screen.getByRole('group', { name: 'Total' })).toHaveClass(FIELD_MARKED_CLASS);
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
    expect(screen.getByRole('button', { name: message('sheet.preview') })).toBeDisabled();
  });

  it('Edge: an edit after a preview clears the marks and re-enables Preview (WSC-24)', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome({ derived: { total: 5 } }),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));
    await screen.findByText(message('engine.modified'));

    await user.clear(level);
    await user.type(level, '6');

    expect(screen.queryByText(message('engine.modified'))).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.preview') })).toBeEnabled();
  });

  it('Edge: a marked row whose value is empty still shows its label and tint (WSC-23)', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome({}),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    const name = await screen.findByRole('textbox', { name: 'Name' });
    await user.clear(name);
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));

    await waitFor(() => expect(previewRequests(fake)).toHaveLength(1));

    expect(await screen.findByText(message('engine.modified'))).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('');
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveClass(FIELD_MARKED_CLASS);
  });
});
