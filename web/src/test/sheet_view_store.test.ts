import { describe, expect, it, vi } from 'vitest';
import {
  DEFAULT_VIEW,
  parseViewRecord,
  serializeViewRecord,
  collapseSections,
  createViewStore,
  expandAllSections,
  expandSection,
  parseCollapsedRecord,
  revealSectionView,
  toggleCollapsed,
  viewStorageKey,
  withOnlyPinned,
} from '../sheet/view_store';
import { createMemoryViewStorage } from './view_storage';

const KEYS = ['identity', 'abilities', 'skills'];
const SHEET = 'sheet-1';
const KNOWN = new Set(KEYS);

function seedStore(seed: Record<string, string> = {}) {
  const storage = createMemoryViewStorage(seed);
  const store = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage });
  return { storage, store };
}

describe('parseCollapsedRecord', () => {
  const tests: { name: string; raw: string | null; expected: string[] }[] = [
    { name: 'Edge: no record reads as nothing collapsed', raw: null, expected: [] },
    { name: 'Edge: corrupt JSON falls back to defaults', raw: '{not json', expected: [] },
    { name: 'Edge: a JSON scalar falls back to defaults', raw: '7', expected: [] },
    { name: 'Edge: null JSON falls back to defaults', raw: 'null', expected: [] },
    {
      name: 'Edge: a wrong version falls back to defaults',
      raw: JSON.stringify({ v: 2, collapsed: ['identity'] }),
      expected: [],
    },
    {
      name: 'Edge: a missing version falls back to defaults',
      raw: JSON.stringify({ collapsed: ['identity'] }),
      expected: [],
    },
    {
      name: 'Edge: collapsed that is not a list falls back to defaults',
      raw: JSON.stringify({ v: 1, collapsed: 'identity' }),
      expected: [],
    },
    {
      name: 'Edge: non-string entries are ignored',
      raw: JSON.stringify({ v: 1, collapsed: [1, null, {}, 'skills'] }),
      expected: ['skills'],
    },
    {
      name: 'Edge: stale keys are ignored',
      raw: JSON.stringify({ v: 1, collapsed: ['gone', 'identity'] }),
      expected: ['identity'],
    },
    {
      name: 'Edge: duplicate keys keep the first',
      raw: JSON.stringify({ v: 1, collapsed: ['skills', 'identity', 'skills'] }),
      expected: ['skills', 'identity'],
    },
    {
      name: 'Happy path: a valid record is read in order',
      raw: JSON.stringify({ v: 1, collapsed: ['abilities', 'identity'] }),
      expected: ['abilities', 'identity'],
    },
  ];

  for (const tc of tests) {
    it(tc.name, () => {
      expect(parseCollapsedRecord(tc.raw, KNOWN)).toEqual(tc.expected);
    });
  }
});

describe('the pure view operations', () => {
  it('Happy path: toggling twice returns to the starting collapsed set', () => {
    const once = toggleCollapsed(DEFAULT_VIEW, 'identity');
    const twice = toggleCollapsed(once, 'identity');

    expect(once.collapsed).toEqual(['identity']);
    expect(twice.collapsed).toEqual([]);
  });

  it('Edge: expanding a Section that is not collapsed returns the same reference', () => {
    expect(expandSection(DEFAULT_VIEW, 'identity')).toBe(DEFAULT_VIEW);
  });

  it('Edge: collapsing Sections that all already are returns the same reference', () => {
    const state = collapseSections(DEFAULT_VIEW, KEYS);

    expect(state.collapsed).toEqual(KEYS);
    expect(collapseSections(state, KEYS)).toBe(state);
  });

  it('Edge: collapsing adds only the missing Sections and keeps the existing order', () => {
    const state = collapseSections(toggleCollapsed(DEFAULT_VIEW, 'skills'), KEYS);

    expect(state.collapsed).toEqual(['skills', 'identity', 'abilities']);
  });

  it('Edge: expanding all when none is collapsed returns the same reference', () => {
    expect(expandAllSections(DEFAULT_VIEW)).toBe(DEFAULT_VIEW);
  });

  it('Edge: the filter setter returns the same reference when it already holds the value', () => {
    expect(withOnlyPinned(DEFAULT_VIEW, false)).toBe(DEFAULT_VIEW);
    expect(withOnlyPinned(DEFAULT_VIEW, true).onlyPinned).toBe(true);
  });

  it('Happy path: revealing a Section turns the filter off and expands it', () => {
    const state = withOnlyPinned(toggleCollapsed(DEFAULT_VIEW, 'skills'), true);
    const revealed = revealSectionView(state, 'skills');

    expect(revealed).toEqual({
      collapsed: [],
      onlyPinned: false,
      explainerSeen: false,
      settingsOpen: false,
    });
  });

  it('Edge: revealing an already visible Section returns the same reference', () => {
    expect(revealSectionView(DEFAULT_VIEW, 'skills')).toBe(DEFAULT_VIEW);
  });
});

describe('the view store', () => {
  it('Happy path: starts expanded with the filter off when nothing is remembered', () => {
    const { store } = seedStore();

    expect(store.getState()).toEqual({
      collapsed: [],
      onlyPinned: false,
      explainerSeen: false,
      settingsOpen: false,
    });
  });

  it('Happy path: restores the collapsed Sections remembered for this sheet', () => {
    const { store } = seedStore({
      [viewStorageKey(SHEET)]: JSON.stringify({ v: 1, collapsed: ['abilities'] }),
    });

    expect(store.getState().collapsed).toEqual(['abilities']);
  });

  it('Edge: a record for another sheet is not read', () => {
    const { store } = seedStore({
      [viewStorageKey('sheet-2')]: JSON.stringify({ v: 1, collapsed: ['abilities'] }),
    });

    expect(store.getState().collapsed).toEqual([]);
  });

  it('Edge: toggling remembers per sheet and leaves another sheet untouched', () => {
    const { storage, store } = seedStore();

    store.toggle('identity');

    expect(JSON.parse(storage.records.get(viewStorageKey(SHEET)) ?? 'null')).toEqual({
      v: 1,
      collapsed: ['identity'],
      onlyPinned: false,
      explainerSeen: false,
      settingsOpen: false,
    });
    expect(storage.records.has(viewStorageKey('sheet-2'))).toBe(false);
  });

  it('Edge: toggling an unknown key changes and writes nothing', () => {
    const { storage, store } = seedStore();
    const before = store.getState();

    store.toggle('ghost');

    expect(store.getState()).toBe(before);
    expect(storage.records.size).toBe(0);
  });

  it('Edge: collapseAll and expandAll are idempotent and keep the same reference', () => {
    const { store } = seedStore();

    store.collapseAll();
    const collapsed = store.getState();
    store.collapseAll();
    expect(store.getState()).toBe(collapsed);
    expect(collapsed.collapsed).toEqual(KEYS);

    store.expandAll();
    const expanded = store.getState();
    store.expandAll();
    expect(store.getState()).toBe(expanded);
    expect(expanded.collapsed).toEqual([]);
  });

  it('Edge: collapse changes are ignored while the filter is on, and restored when it is off', () => {
    const { store } = seedStore();
    store.toggle('identity');
    store.setOnlyPinned(true);

    store.toggle('abilities');
    store.collapseAll();
    store.expandAll();

    expect(store.getState().collapsed).toEqual(['identity']);
    store.setOnlyPinned(false);
    expect(store.getState().collapsed).toEqual(['identity']);
  });

  it('Happy path: the filter is remembered per sheet and read back on the next open', () => {
    const { storage, store } = seedStore();

    store.setOnlyPinned(true);

    expect(JSON.parse(storage.records.get(viewStorageKey(SHEET)) ?? 'null')).toEqual({
      v: 1,
      collapsed: [],
      onlyPinned: true,
      explainerSeen: false,
      settingsOpen: false,
    });
    const reopened = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage });
    expect(reopened.getState().onlyPinned).toBe(true);
  });

  it('Edge: the filter of sheet A does not leak into sheet B', () => {
    const { storage, store } = seedStore();
    store.setOnlyPinned(true);

    const other = createViewStore({ sheetId: 'sheet-2', sectionKeys: KEYS, storage });

    expect(other.getState().onlyPinned).toBe(false);
    expect(storage.records.has(viewStorageKey('sheet-2'))).toBe(false);
  });

  it('Edge: turning the filter off is remembered too', () => {
    const { storage, store } = seedStore();
    store.setOnlyPinned(true);
    store.setOnlyPinned(false);

    const reopened = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage });

    expect(reopened.getState().onlyPinned).toBe(false);
  });

  it('Edge: setting the filter to its current value writes nothing', () => {
    const { storage, store } = seedStore();

    store.setOnlyPinned(false);

    expect(storage.records.size).toBe(0);
  });

  it('Edge: a storage that throws keeps the filter in memory only', () => {
    const storage = createMemoryViewStorage({}, { throws: true });
    const store = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage });

    expect(() => store.setOnlyPinned(true)).not.toThrow();
    expect(store.getState().onlyPinned).toBe(true);
  });

  it('Edge: reveal of a collapsed Section under the filter turns the filter off and expands it', () => {
    const { store } = seedStore();
    store.toggle('skills');
    store.setOnlyPinned(true);

    store.reveal('skills');

    expect(store.getState()).toEqual({
      collapsed: [],
      onlyPinned: false,
      explainerSeen: false,
      settingsOpen: false,
    });
  });

  it('Edge: reveal of an unknown key changes nothing when the filter is already off', () => {
    const { store } = seedStore();
    store.toggle('identity');
    const before = store.getState();

    store.reveal('ghost');

    expect(store.getState()).toBe(before);
  });

  it('Edge: reveal of an unknown key still turns the filter off', () => {
    const { store } = seedStore();
    store.setOnlyPinned(true);

    store.reveal('ghost');

    expect(store.getState().onlyPinned).toBe(false);
  });

  it('Edge: a storage that throws on read yields defaults without raising', () => {
    const storage = createMemoryViewStorage({}, { throws: true });

    const store = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage });

    expect(store.getState()).toEqual(DEFAULT_VIEW);
  });

  it('Edge: a storage that throws on write keeps the in-memory state and raises nothing', () => {
    const storage = createMemoryViewStorage({}, { throws: true });
    const store = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage });

    expect(() => store.toggle('identity')).not.toThrow();
    expect(store.getState().collapsed).toEqual(['identity']);
  });

  it('Edge: with no storage at all the store still works in memory', () => {
    const store = createViewStore({ sheetId: SHEET, sectionKeys: KEYS, storage: undefined });

    store.toggle('skills');

    expect(store.getState().collapsed).toEqual(['skills']);
  });

  it('Edge: subscribers hear a change once and a no-op never', () => {
    const { store } = seedStore();
    const listener = vi.fn();
    const unsubscribe = store.subscribe(listener);

    store.toggle('identity');
    store.expandAll();
    store.expandAll();
    store.setOnlyPinned(false);

    expect(listener).toHaveBeenCalledTimes(2);
    unsubscribe();
    store.toggle('identity');
    expect(listener).toHaveBeenCalledTimes(2);
  });
});

describe('parseViewRecord', () => {
  const tests: { name: string; raw: string | null; expected: unknown }[] = [
    { name: 'Edge: no record is the default view', raw: null, expected: DEFAULT_VIEW },
    { name: 'Edge: corrupt JSON is the default view', raw: '{nope', expected: DEFAULT_VIEW },
    { name: 'Edge: a scalar is the default view', raw: '3', expected: DEFAULT_VIEW },
    {
      name: 'Edge: a wrong version drops the filter too',
      raw: JSON.stringify({ v: 2, collapsed: ['identity'], onlyPinned: true }),
      expected: DEFAULT_VIEW,
    },
    {
      name: 'Edge: a record from before the filter was remembered reads as off',
      raw: JSON.stringify({ v: 1, collapsed: ['identity'] }),
      expected: {
        collapsed: ['identity'],
        onlyPinned: false,
        explainerSeen: false,
        settingsOpen: false,
      },
    },
    {
      name: 'Edge: a non-boolean filter reads as off and keeps the collapsed list',
      raw: JSON.stringify({ v: 1, collapsed: ['skills'], onlyPinned: 'yes' }),
      expected: {
        collapsed: ['skills'],
        onlyPinned: false,
        explainerSeen: false,
        settingsOpen: false,
      },
    },
    {
      name: 'Edge: a bad collapsed list keeps a valid filter',
      raw: JSON.stringify({ v: 1, collapsed: 'x', onlyPinned: true }),
      expected: { collapsed: [], onlyPinned: true, explainerSeen: false, settingsOpen: false },
    },
    {
      name: 'Edge: stale Sections are dropped, the filter stays',
      raw: JSON.stringify({ v: 1, collapsed: ['gone'], onlyPinned: true }),
      expected: { collapsed: [], onlyPinned: true, explainerSeen: false, settingsOpen: false },
    },
    {
      name: 'Happy path: a full record round-trips',
      raw: serializeViewRecord({
        collapsed: ['abilities'],
        onlyPinned: true,
        explainerSeen: false,
        settingsOpen: false,
      }),
      expected: {
        collapsed: ['abilities'],
        onlyPinned: true,
        explainerSeen: false,
        settingsOpen: false,
      },
    },
  ];

  for (const tc of tests) {
    it(tc.name, () => {
      expect(parseViewRecord(tc.raw, KNOWN)).toEqual(tc.expected);
    });
  }
});
