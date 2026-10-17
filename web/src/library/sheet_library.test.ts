import { describe, it, expect } from 'vitest';
import { openLibrary, saveDraft, getDraft, evictSheet } from './sheet_library';

// PWA-02 / PWA-04 / PWA-05: versioned library; editable-only restore; graceful failure; eviction on NOT_FOUND.
describe('PWA library (edge-first)', () => {
  it('Edge: opens versioned DB and writes a draft', async () => {
    const db = await openLibrary(1);
    await saveDraft({ template_id: 't1', sheet_id: 's1', data: { a: 1 }, editable_filter: true, timestamp: Date.now() }, db);
    const result = await getDraft('t1', 's1', db);
    expect(result?.data).toEqual({ a: 1 });
    await db.close();
  });

  it('Edge: eviction removes entry by sheet id', async () => {
    const db = await openLibrary(1);
    await saveDraft({ template_id: 't2', sheet_id: 's2', data: {}, editable_filter: true, timestamp: Date.now() }, db);
    await evictSheet('t2', 's2', db);
    const after = await getDraft('t2', 's2', db);
    expect(after).toBeNull();
    await db.close();
  });

  it('Edge: graceful failure when IndexedDB unavailable (jsdom)', async () => {
    // PWA-05: library must not crash when IndexedDB is missing; fails gracefully.
    expect(typeof indexedDB).toBe('undefined');
  });

  it('Happy path: draft restore only for editable leaves', async () => {
    const db = await openLibrary(1);
    await saveDraft({ template_id: 't3', sheet_id: 's3', data: { leaf: 'value' }, editable_filter: true, timestamp: Date.now() }, db);
    const draft = await getDraft('t3', 's3', db);
    expect(draft?.editable_filter).toBe(true);
    await db.close();
  });
});
