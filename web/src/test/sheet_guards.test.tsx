import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import type { Transport } from '../api/transport';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import { canPreview, canSave, createSheetEditStore } from '../sheet/sheet_edit';
import { createDeferredTransport, createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

const SHEET_URL = `/sheets/${SHEET_ID}`;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

interface ManualClock {
  readonly now: () => number;
  readonly schedule: (callback: () => void, delayMs: number) => () => void;
  advance(ms: number): void;
}

/** A hand-driven clock so the rolling window's boundaries are exercised exactly (WSC-38). */
function manualClock(): ManualClock {
  let current = 0;
  const timers: Array<{ at: number; callback: () => void }> = [];

  return {
    now: () => current,
    schedule: (callback, delayMs) => {
      const entry = { at: current + delayMs, callback };
      timers.push(entry);
      return () => {
        const index = timers.indexOf(entry);
        if (index >= 0) {
          timers.splice(index, 1);
        }
      };
    },
    advance: (ms) => {
      current += ms;
      for (const entry of [...timers]) {
        if (entry.at <= current) {
          timers.splice(timers.indexOf(entry), 1);
          entry.callback();
        }
      }
    },
  };
}

function throttledStore(clock: ManualClock) {
  const fake = createFakeTransport(successOutcome({}));
  const store = createSheetEditStore(
    {
      template: parseTemplateDetail(templateDetailPayload()),
      sheet: parseSheet(sheetPayload()),
    },
    {
      client: clientWith(fake.transport),
      store: createMemorySheetStore(),
      now: clock.now,
      schedule: clock.schedule,
    },
  );
  return { store, fake };
}

describe('request throttle (WSC-38, SC-011)', () => {
  it('Edge: a fourth Save inside the rolling minute issues no request and re-enables once it frees (WSC-38)', async () => {
    const clock = manualClock();
    const { store, fake } = throttledStore(clock);
    const patches = () => fake.requests.filter((request) => request.method === 'PATCH').length;

    for (let index = 0; index < 3; index += 1) {
      store.setPath('identity.level', 4 + index);
      await store.save();
    }

    expect(patches()).toBe(3);

    store.setPath('identity.level', 100);
    expect(canSave(store.getState())).toBe(false);
    await store.save();

    expect(patches()).toBe(3);

    clock.advance(60_000);

    expect(canSave(store.getState())).toBe(true);
    await store.save();

    expect(patches()).toBe(4);
  });

  it('Edge: a fourth Preview inside the rolling minute issues no request and re-enables once it frees (WSC-38)', async () => {
    const clock = manualClock();
    const { store, fake } = throttledStore(clock);
    const previews = () =>
      fake.requests.filter((request) => request.url.endsWith('/preview')).length;

    for (let index = 0; index < 3; index += 1) {
      store.setPath('identity.level', 4 + index);
      await store.preview();
    }

    expect(previews()).toBe(3);

    store.setPath('identity.level', 100);
    expect(canPreview(store.getState())).toBe(false);
    await store.preview();

    expect(previews()).toBe(3);

    clock.advance(60_000);

    expect(canPreview(store.getState())).toBe(true);
    await store.preview();

    expect(previews()).toBe(4);
  });

  it('Edge: the throttle does not replace the in-flight no-op rule (WSC-38)', async () => {
    const clock = manualClock();
    const { store } = throttledStore(clock);

    store.setPath('identity.level', 4);

    expect(canSave(store.getState())).toBe(true);
    expect(canPreview(store.getState())).toBe(true);
  });
});

describe('keyboard and leave guards (WSC-20, WSC-36, WSC-37)', () => {
  async function openSheet(transport: Transport) {
    const user = userEvent.setup();
    const view = renderApp(SHEET_URL, { client: clientWith(transport) });
    const level = await screen.findByRole('textbox', { name: 'Level' });
    return { user, level, sheetStore: view.sheetStore };
  }

  it('Edge: Ctrl+S with no sheet screen mounted runs nothing (WSC-20)', async () => {
    const fake = createFakeTransport(successOutcome([]));
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(fake.transport) });
    await screen.findByTestId('catalog');

    await user.keyboard('{Control>}s{/Control}');

    expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(0);
  });

  it('Edge: F5 while a save is in flight shows no confirmation and issues no request (WSC-36)', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp(SHEET_URL, { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(sheetPayload()));
    expect(await screen.findByText(message('sheet.templateLoading'))).toBeInTheDocument();
    deferred.resolve(successOutcome(templateDetailPayload()));
    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');

    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    const templates = () =>
      deferred.requests.filter((request) => request.url.includes('/templates/')).length;
    const before = templates();

    await user.keyboard('{F5}');

    expect(screen.queryByTestId('sheet-confirm-refresh')).not.toBeInTheDocument();
    expect(templates()).toBe(before);

    deferred.resolve(successOutcome({}));
  });

  it('Edge: leaving while a save is in flight is a no-op (WSC-37)', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp(SHEET_URL, { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(sheetPayload()));
    expect(await screen.findByText(message('sheet.templateLoading'))).toBeInTheDocument();
    deferred.resolve(successOutcome(templateDetailPayload()));
    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');

    await user.click(screen.getByRole('button', { name: message('sheet.save') }));
    await user.click(screen.getByRole('link', { name: message('nav.catalog') }));

    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-screen')).toBeInTheDocument();

    deferred.resolve(successOutcome({}));
  });

  it('Edge: a second refresh or a leave activation while a confirmation is open is ignored (WSC-36, WSC-37)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user, level } = await openSheet(fake.transport);
    await user.clear(level);
    await user.type(level, '4');

    await user.keyboard('{F5}');

    expect(screen.getByTestId('sheet-confirm-refresh')).toBeInTheDocument();

    await user.keyboard('{F5}');
    await user.click(screen.getByRole('link', { name: message('nav.catalog') }));

    expect(screen.getAllByTestId('sheet-confirm-refresh')).toHaveLength(1);
    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-screen')).toBeInTheDocument();
  });

  it('Happy path: Ctrl+S runs the same save as the footer action (WSC-20)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome({}),
    );
    const { user, level } = await openSheet(fake.transport);
    await user.clear(level);
    await user.type(level, '4');

    await user.keyboard('{Control>}s{/Control}');

    await waitFor(() => {
      expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(1);
    });
    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
  });

  it('Happy path: F5 on a clean sheet re-fetches the template detail exactly once (WSC-36)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user } = await openSheet(fake.transport);

    await user.keyboard('{F5}');

    await waitFor(() => {
      expect(fake.requests.filter((request) => request.url.includes('/templates/'))).toHaveLength(
        2,
      );
    });
    expect(screen.queryByTestId('sheet-confirm-refresh')).not.toBeInTheDocument();
  });

  it('Happy path: F5 cancel keeps the edits and issues no template request (WSC-36)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user, level } = await openSheet(fake.transport);
    await user.clear(level);
    await user.type(level, '4');
    await user.keyboard('{F5}');

    await user.click(screen.getByRole('button', { name: message('sheet.cancel') }));

    expect(screen.queryByTestId('sheet-confirm-refresh')).not.toBeInTheDocument();
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
    expect(fake.requests.filter((request) => request.url.includes('/templates/'))).toHaveLength(1);
  });

  it('Happy path: Escape cancels the refresh confirmation and keeps the edits (WSC-36, WSC-39)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user, level } = await openSheet(fake.transport);
    await user.clear(level);
    await user.type(level, '4');
    await user.keyboard('{F5}');

    await user.keyboard('{Escape}');

    expect(screen.queryByTestId('sheet-confirm-refresh')).not.toBeInTheDocument();
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
    expect(fake.requests.filter((request) => request.url.includes('/templates/'))).toHaveLength(1);
  });

  it('Happy path: the overlay barrier cancels the leave confirmation and stays on the sheet (WSC-37)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user, level } = await openSheet(fake.transport);
    await user.clear(level);
    await user.type(level, '4');

    await user.click(screen.getByRole('link', { name: message('nav.catalog') }));
    await user.click(screen.getByTestId('sheet-confirm-overlay'));

    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-screen')).toBeInTheDocument();
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  });

  it('Happy path: F5 confirm discards the edits, deletes the draft and re-fetches once (WSC-36, WSC-29)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user, level, sheetStore } = await openSheet(fake.transport);
    await sheetStore.writeDraft(SHEET_ID, { identity: { level: 4 } });
    await user.clear(level);
    await user.type(level, '4');
    await user.keyboard('{F5}');

    await user.click(screen.getByRole('button', { name: message('sheet.refreshConfirm') }));

    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
    await waitFor(() => {
      expect(fake.requests.filter((request) => request.url.includes('/templates/'))).toHaveLength(
        2,
      );
    });
    expect(await sheetStore.readDraft(SHEET_ID)).toBeNull();
  });

  it('Happy path: leaving cancel keeps the edits and stays on the sheet (WSC-37)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    const { user, level } = await openSheet(fake.transport);
    await user.clear(level);
    await user.type(level, '4');

    await user.click(screen.getByRole('link', { name: message('nav.catalog') }));

    expect(screen.getByTestId('sheet-confirm-leave')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.cancel') }));

    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-screen')).toBeInTheDocument();
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  });

  it('Happy path: leaving confirm discards, deletes the draft and navigates (WSC-37, WSC-29)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome([]),
    );
    const { user, level, sheetStore } = await openSheet(fake.transport);
    await sheetStore.writeDraft(SHEET_ID, { identity: { level: 4 } });
    await user.clear(level);
    await user.type(level, '4');

    await user.click(screen.getByRole('link', { name: message('nav.catalog') }));
    await user.click(screen.getByRole('button', { name: message('sheet.leaveConfirm') }));

    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
    expect(await sheetStore.readDraft(SHEET_ID)).toBeNull();
  });

  it('Happy path: a clean sheet leaves with no dialog (WSC-37)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome([]),
    );
    const { user } = await openSheet(fake.transport);

    await user.click(screen.getByRole('link', { name: message('nav.catalog') }));

    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
  });
});
