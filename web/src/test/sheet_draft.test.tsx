import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import { message } from '../l10n/strings';
import { DRAFT_CADENCE_MS } from '../library/draft_backup';
import { SCHEMA_VERSION, createMemorySheetStore } from '../library/sheet_store';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

const SHEET_URL = `/sheets/${SHEET_ID}`;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function loadedFake() {
  return createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(templateDetailPayload()),
  );
}

/** The slice of a `vi.spyOn` result this suite reads, so the calls stay strongly typed. */
interface IntervalSpy {
  readonly mock: {
    readonly calls: ReadonlyArray<readonly unknown[]>;
    readonly results: ReadonlyArray<{ readonly value?: unknown }>;
  };
}

/** Finds the cadence's `setInterval` call among every interval the tree schedules. */
function cadenceCall(spy: IntervalSpy) {
  const index = spy.mock.calls.findIndex((call) => call[1] === DRAFT_CADENCE_MS);
  return { index, tick: spy.mock.calls[index]?.[0] as (() => void) | undefined };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('sheet draft on the mounted screen (WSC-26, WSC-28, WSC-30)', () => {
  it('Happy path: opening overlays the stored draft and marks the restored edit pending (WSC-28)', async () => {
    const sheetStore = createMemorySheetStore();
    await sheetStore.writeDraft(SHEET_ID, { identity: { level: 8 } });
    renderApp(SHEET_URL, { client: clientWith(loadedFake().transport), sheetStore });

    const level = await screen.findByRole('textbox', { name: 'Level' });

    await waitFor(() => expect(level).toHaveValue('8'));
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  });

  it('Edge: a corrupt stored draft leaves the sheet clean (WSC-30)', async () => {
    const sheetStore = createMemorySheetStore();
    sheetStore.putRawDraft(SHEET_ID, {
      schemaVersion: SCHEMA_VERSION,
      id: SHEET_ID,
      delta: 'nope',
    });
    renderApp(SHEET_URL, { client: clientWith(loadedFake().transport), sheetStore });

    const level = await screen.findByRole('textbox', { name: 'Level' });

    expect(level).toHaveValue('3');
    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
  });

  it('Happy path: the mounted cadence backs up the pending editable delta (WSC-26)', async () => {
    const interval = vi.spyOn(globalThis, 'setInterval');
    const sheetStore = createMemorySheetStore();
    const user = userEvent.setup();
    renderApp(SHEET_URL, { client: clientWith(loadedFake().transport), sheetStore });

    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');

    const { tick } = cadenceCall(interval);
    expect(tick).toBeDefined();
    tick?.();

    await waitFor(async () => {
      expect(await sheetStore.readDraft(SHEET_ID)).toEqual({ identity: { level: 4 } });
    });
  });

  it('Edge: unmounting the screen clears the cadence interval (WSC-26)', async () => {
    const interval = vi.spyOn(globalThis, 'setInterval');
    const clear = vi.spyOn(globalThis, 'clearInterval');
    const view = renderApp(SHEET_URL, { client: clientWith(loadedFake().transport) });
    await screen.findByRole('textbox', { name: 'Level' });

    const { index } = cadenceCall(interval);
    const handle = interval.mock.results[index]?.value;

    view.unmount();

    expect(clear).toHaveBeenCalledWith(handle);
  });
});
