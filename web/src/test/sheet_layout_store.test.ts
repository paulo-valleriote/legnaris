import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from '../api/client';
import type { Layout } from '../api/models/layout';
import { moveSection, pinRow, unpinRow } from '../engine/arrange_ops';
import type { Arrangement } from '../engine/layout';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import type { LayoutStore } from '../sheet/layout_store';
import { createLayoutStore, isArranging, isLayoutPending } from '../sheet/layout_store';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { arrangeSections } from './layout_fixtures';

const SHEET = '01a0f967-f42b-710e-851f-8f293a634921';
const sections = arrangeSections();

function moveSectionBy(arrangement: Arrangement, key: string, delta: number): Arrangement {
  return moveSection(arrangement, key, arrangement.sectionOrder.indexOf(key) + delta);
}

function build(
  outcomes: Parameters<typeof createFakeTransport>,
  layout?: Layout,
  onCommitted?: (layout: Layout | null) => void,
) {
  const fake = createFakeTransport(...outcomes);
  const store = createMemorySheetStore();
  const layoutStore = createLayoutStore(
    { sheetId: SHEET, sections, layout },
    {
      client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
      store,
      onCommitted,
    },
  );
  return { fake, store, layoutStore };
}

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

function puts(fake: ReturnType<typeof createFakeTransport>) {
  return fake.requests.filter((request) => request.method === 'PUT');
}

describe('layout store lifecycle', () => {
  it('Edge: starts outside Arrange mode with the Template order committed and nothing pending', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    const state = layoutStore.getState();

    expect(isArranging(state)).toBe(false);
    expect(isLayoutPending(state)).toBe(false);
    expect(state.committed.sectionOrder).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Happy path: a committed layout seeds the committed arrangement', () => {
    const { layoutStore } = build([successOutcome({ layout: null })], {
      sectionOrder: ['gamma'],
      pinned: ['alpha.one'],
    });

    expect(layoutStore.getState().committed.sectionOrder).toEqual(['gamma', 'alpha', 'beta']);
    expect(layoutStore.getState().committed.pinned).toEqual(['alpha.one']);
  });

  it('Edge: enter stages the committed arrangement without making anything pending', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);

    layoutStore.enter();

    expect(isArranging(layoutStore.getState())).toBe(true);
    expect(isLayoutPending(layoutStore.getState())).toBe(false);
  });

  it('Edge: apply and reset are no-ops outside Arrange mode', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    const listener = vi.fn();
    layoutStore.subscribe(listener);

    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    layoutStore.reset();

    expect(listener).not.toHaveBeenCalled();
    expect(isArranging(layoutStore.getState())).toBe(false);
  });

  it('Edge: an operation that changes nothing publishes nothing', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();
    const listener = vi.fn();
    layoutStore.subscribe(listener);

    layoutStore.apply((staged) => moveSectionBy(staged, 'alpha', -1));

    expect(listener).not.toHaveBeenCalled();
  });

  it('Edge: staging the same pin twice publishes once and stays idempotent', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();
    const listener = vi.fn();
    layoutStore.subscribe(listener);

    layoutStore.apply((staged) => pinRow(staged, 'alpha.two'));
    layoutStore.apply((staged) => pinRow(staged, 'alpha.two'));

    expect(listener).toHaveBeenCalledTimes(1);
    expect(layoutStore.getState().staged?.pinned).toEqual(['alpha.two']);
  });

  it('Edge: a staged pin naming no Row is dropped, so nothing becomes pending', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();
    const listener = vi.fn();
    layoutStore.subscribe(listener);

    layoutStore.apply((staged) => pinRow(staged, 'alpha.ghost'));

    expect(listener).not.toHaveBeenCalled();
    expect(isLayoutPending(layoutStore.getState())).toBe(false);
  });

  it('Edge: pinning keeps the pin order and unpinning returns to the committed state', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();

    layoutStore.apply((staged) => pinRow(staged, 'gamma.solo'));
    layoutStore.apply((staged) => pinRow(staged, 'alpha.one'));
    expect(layoutStore.getState().staged?.pinned).toEqual(['gamma.solo', 'alpha.one']);

    layoutStore.apply((staged) => unpinRow(unpinRow(staged, 'gamma.solo'), 'alpha.one'));
    expect(isLayoutPending(layoutStore.getState())).toBe(false);
  });

  it('Edge: unsubscribe stops notifications', () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    const listener = vi.fn();
    const unsubscribe = layoutStore.subscribe(listener);
    unsubscribe();

    layoutStore.enter();

    expect(listener).not.toHaveBeenCalled();
  });
});

describe('commit', () => {
  it('Happy path: Done sends exactly one PUT with the deviating lists and adopts the answer', async () => {
    const onCommitted = vi.fn();
    const stored = { section_order: ['beta', 'alpha', 'gamma'], pinned: ['gamma.solo'] };
    const { fake, layoutStore } = build(
      [successOutcome({ layout: stored })],
      undefined,
      onCommitted,
    );
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    layoutStore.apply((staged) => pinRow(staged, 'gamma.solo'));

    await layoutStore.commit();

    expect(puts(fake)).toHaveLength(1);
    expect(fake.requests).toHaveLength(1);
    expect(puts(fake)[0].url).toBe(`http://api.test/sheets/${SHEET}/layout`);
    expect(JSON.parse(puts(fake)[0].body ?? '')).toEqual({ layout: stored });
    const state = layoutStore.getState();
    expect(isArranging(state)).toBe(false);
    expect(state.committed.sectionOrder).toEqual(['beta', 'alpha', 'gamma']);
    expect(state.committed.pinned).toEqual(['gamma.solo']);
    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith({
      sectionOrder: ['beta', 'alpha', 'gamma'],
      pinned: ['gamma.solo'],
    });
  });

  it('Happy path: pins alone are sent in pin order, without a section list', async () => {
    const { fake, layoutStore } = build([
      successOutcome({ layout: { pinned: ['gamma.solo', 'alpha.one'] } }),
    ]);
    layoutStore.enter();
    layoutStore.apply((staged) => pinRow(staged, 'gamma.solo'));
    layoutStore.apply((staged) => pinRow(staged, 'alpha.one'));

    await layoutStore.commit();

    expect(puts(fake)[0].body).toBe('{"layout":{"pinned":["gamma.solo","alpha.one"]}}');
  });

  it('Edge: the PUT body never carries row_order', async () => {
    const { fake, layoutStore } = build([successOutcome({ layout: { pinned: ['alpha.one'] } })]);
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    layoutStore.apply((staged) => pinRow(staged, 'alpha.one'));

    await layoutStore.commit();

    expect(puts(fake)[0].body).not.toContain('row_order');
  });

  it('Edge: Done with no change leaves Arrange mode and sends nothing', async () => {
    const { fake, layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();

    await layoutStore.commit();

    expect(fake.attempts).toBe(0);
    expect(isArranging(layoutStore.getState())).toBe(false);
  });

  it('Edge: moving and moving back is not pending and sends nothing', async () => {
    const { fake, layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    expect(isLayoutPending(layoutStore.getState())).toBe(true);
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', 1));
    expect(isLayoutPending(layoutStore.getState())).toBe(false);

    await layoutStore.commit();

    expect(fake.attempts).toBe(0);
  });

  it('Edge: Reset stages the Template default and Done sends {"layout": null}', async () => {
    const onCommitted = vi.fn();
    const { fake, layoutStore } = build(
      [successOutcome({ layout: null })],
      { sectionOrder: ['gamma'], pinned: ['alpha.one'] },
      onCommitted,
    );
    layoutStore.enter();
    layoutStore.reset();

    expect(isLayoutPending(layoutStore.getState())).toBe(true);
    expect(fake.attempts).toBe(0);

    await layoutStore.commit();

    expect(puts(fake)).toHaveLength(1);
    expect(JSON.parse(puts(fake)[0].body ?? '')).toEqual({ layout: null });
    expect(layoutStore.getState().committed.pinned).toEqual([]);
    expect(onCommitted).toHaveBeenCalledWith(null);
  });

  it('Edge: Reset on an already default arrangement is not pending', async () => {
    const { layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();
    layoutStore.reset();

    expect(isLayoutPending(layoutStore.getState())).toBe(false);
  });

  it('Edge: a second Done while the PUT is in flight sends no second request', async () => {
    const deferred = createDeferredTransport();
    const layoutStore = createLayoutStore(
      { sheetId: SHEET, sections, layout: undefined },
      {
        client: createApiClient({ baseUrl: 'http://api.test', transport: deferred.transport }),
        store: createMemorySheetStore(),
      },
    );
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));

    const first = layoutStore.commit();
    const second = layoutStore.commit();
    layoutStore.apply((staged) => moveSectionBy(staged, 'gamma', -1));
    layoutStore.cancel();
    layoutStore.enter();

    expect(layoutStore.getState().isCommitting).toBe(true);
    expect(isArranging(layoutStore.getState())).toBe(true);
    deferred.resolve({
      status: 200,
      body: JSON.stringify({ data: { layout: { section_order: ['beta', 'alpha', 'gamma'] } } }),
    });
    await Promise.all([first, second]);

    expect(deferred.requests).toHaveLength(1);
    expect(layoutStore.getState().isCommitting).toBe(false);
  });

  it('Error: a failed PUT keeps the staging, maps the copy and allows a retry', async () => {
    const onCommitted = vi.fn();
    const { fake, layoutStore } = build(
      [
        errorOutcome('INTERNAL_ERROR', 'boom', 500),
        successOutcome({ layout: { section_order: ['beta', 'alpha', 'gamma'] } }),
      ],
      undefined,
      onCommitted,
    );
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));

    await layoutStore.commit();

    let state = layoutStore.getState();
    expect(isArranging(state)).toBe(true);
    expect(isLayoutPending(state)).toBe(true);
    expect(state.isCommitting).toBe(false);
    expect(state.commitError).toBe(message('api.error.serverError'));
    expect(onCommitted).not.toHaveBeenCalled();

    await layoutStore.commit();

    state = layoutStore.getState();
    expect(puts(fake)).toHaveLength(2);
    expect(isArranging(state)).toBe(false);
    expect(state.commitError).toBeUndefined();
    expect(onCommitted).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['404 NOT_FOUND', errorOutcome('NOT_FOUND', 'gone', 404), 'api.error.notFound'],
    ['400 INVALID_REQUEST', errorOutcome('INVALID_REQUEST', 'bad', 400), 'api.error.invalidEdit'],
    ['401 UNAUTHORIZED', errorOutcome('UNAUTHORIZED', 'no', 401), 'api.error.unauthorized'],
    ['an unreadable answer', successOutcome({}), 'api.error.parse'],
    ['a transport failure', { failure: new Error('down') }, 'api.error.offline'],
  ] as const)(
    'Error: %s keeps the staging and shows its mapped copy',
    async (_name, outcome, key) => {
      const { layoutStore } = build([outcome]);
      layoutStore.enter();
      layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));

      await layoutStore.commit();

      expect(layoutStore.getState().commitError).toBe(message(key));
      expect(isLayoutPending(layoutStore.getState())).toBe(true);
    },
  );

  it('Edge: a further change clears the failure copy', async () => {
    const { layoutStore } = build([errorOutcome('INTERNAL_ERROR', 'boom', 500)]);
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    await layoutStore.commit();

    layoutStore.apply((staged) => moveSectionBy(staged, 'gamma', -1));

    expect(layoutStore.getState().commitError).toBeUndefined();
  });

  it('Edge: a throwing onCommitted never reverts the committed state', async () => {
    const { layoutStore } = build(
      [successOutcome({ layout: { section_order: ['beta', 'alpha', 'gamma'] } })],
      undefined,
      () => {
        throw new Error('cache');
      },
    );
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));

    await layoutStore.commit();

    expect(layoutStore.getState().committed.sectionOrder).toEqual(['beta', 'alpha', 'gamma']);
  });
});

describe('cancel', () => {
  it('Happy path: Cancel discards the staging and sends nothing', async () => {
    const { fake, layoutStore } = build([successOutcome({ layout: null })]);
    layoutStore.enter();
    layoutStore.apply((staged) => pinRow(staged, 'alpha.two'));

    layoutStore.cancel();

    expect(isArranging(layoutStore.getState())).toBe(false);
    expect(layoutStore.getState().committed.pinned).toEqual([]);
    expect(fake.attempts).toBe(0);
  });

  it('Edge: Cancel outside Arrange mode and after a failure are harmless', async () => {
    const { layoutStore } = build([errorOutcome('INTERNAL_ERROR', 'boom', 500)]);

    layoutStore.cancel();
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    await layoutStore.commit();
    layoutStore.cancel();

    expect(isArranging(layoutStore.getState())).toBe(false);
    expect(layoutStore.getState().commitError).toBeUndefined();
  });
});

describe('pending arrangement backup', () => {
  async function drive(layoutStore: LayoutStore) {
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    await flush();
  }

  it('Happy path: every staged change backs up the serialized layout, without any credential', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })]);

    await drive(layoutStore);

    await expect(store.readLayoutDraft(SHEET)).resolves.toEqual({
      layout: { sectionOrder: ['beta', 'alpha', 'gamma'], pinned: [] },
    });
    expect(JSON.stringify(store.records())).not.toMatch(
      /token|password|secret|authorization|cookie|credential/i,
    );
  });

  it('Edge: a staged reset is backed up as a null layout', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })], {
      sectionOrder: ['gamma'],
      pinned: [],
    });
    layoutStore.enter();
    layoutStore.reset();
    await flush();

    await expect(store.readLayoutDraft(SHEET)).resolves.toEqual({ layout: null });
  });

  it('Edge: returning to the committed arrangement, cancel and a successful commit remove the backup', async () => {
    const { store, layoutStore } = build([
      successOutcome({ layout: { section_order: ['beta', 'alpha', 'gamma'] } }),
    ]);
    await drive(layoutStore);
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', 1));
    await flush();
    await expect(store.readLayoutDraft(SHEET)).resolves.toBeNull();

    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    await flush();
    layoutStore.cancel();
    await flush();
    await expect(store.readLayoutDraft(SHEET)).resolves.toBeNull();

    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    await layoutStore.commit();
    await expect(store.readLayoutDraft(SHEET)).resolves.toBeNull();
  });

  it('Edge: a failing backup write never blocks staging', async () => {
    const fake = createFakeTransport(successOutcome({ layout: null }));
    const store = createMemorySheetStore();
    store.writeLayoutDraft = () => Promise.reject(new Error('quota'));
    const layoutStore = createLayoutStore(
      { sheetId: SHEET, sections, layout: undefined },
      { client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }), store },
    );

    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));
    await flush();

    expect(isLayoutPending(layoutStore.getState())).toBe(true);
  });

  it('Happy path: restore re-enters Arrange mode with the backed-up arrangement', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })]);
    await store.writeLayoutDraft(SHEET, {
      sectionOrder: ['gamma'],
      pinned: ['alpha.two'],
    });

    await layoutStore.restore();

    const state = layoutStore.getState();
    expect(isArranging(state)).toBe(true);
    expect(isLayoutPending(state)).toBe(true);
    expect(state.staged?.sectionOrder).toEqual(['gamma', 'alpha', 'beta']);
    expect(state.staged?.pinned).toEqual(['alpha.two']);
  });

  it('Edge: restore ignores a missing, unreadable or already committed draft', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })]);

    await layoutStore.restore();
    expect(isArranging(layoutStore.getState())).toBe(false);

    store.readLayoutDraft = () => Promise.reject(new Error('idb'));
    await layoutStore.restore();
    expect(isArranging(layoutStore.getState())).toBe(false);
  });

  it('Edge: a backed-up arrangement equal to the committed one is dropped, not restored', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })]);
    await store.writeLayoutDraft(SHEET, null);

    await layoutStore.restore();

    expect(isArranging(layoutStore.getState())).toBe(false);
    await expect(store.readLayoutDraft(SHEET)).resolves.toBeNull();
  });

  it('Edge: restore never overwrites an arrangement already being staged', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })]);
    await store.writeLayoutDraft(SHEET, { sectionOrder: ['gamma'], pinned: [] });
    layoutStore.enter();
    layoutStore.apply((staged) => moveSectionBy(staged, 'beta', -1));

    await layoutStore.restore();

    expect(layoutStore.getState().staged?.sectionOrder).toEqual(['beta', 'alpha', 'gamma']);
  });

  it('Edge: a stale backup restores against the current Template, ignoring unknown keys', async () => {
    const { store, layoutStore } = build([successOutcome({ layout: null })]);
    await store.writeLayoutDraft(SHEET, {
      sectionOrder: ['ghost', 'beta'],
      pinned: ['ghost.row'],
    });

    await layoutStore.restore();

    expect(layoutStore.getState().staged?.sectionOrder).toEqual(['beta', 'alpha', 'gamma']);
    expect(layoutStore.getState().staged?.pinned).toEqual([]);
  });
});
