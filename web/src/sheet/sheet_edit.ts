/**
 * The in-memory edit store for one open sheet (WSC-12 … WSC-20, WSC-34).
 *
 * It holds the seeded values, the set of edited paths, the save/preview
 * lifecycle and the preview marks, and issues exactly the `PATCH` a save sends.
 * Every write produces a fresh data tree, the pending delta is rebuilt through
 * the F4 engine's `setPathValue`, and the seed and merge come from F4's
 * `seedFromSheet`/`mergeCascade` — the store defines no second draft, seed or
 * merge. It owns no React state and binds no listener.
 */

import type { ApiClient } from '../api/client';
import { failureCopy } from '../api/error_mapper';
import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { FieldNode, FieldSchema, TemplateDetail } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { buildSections } from '../engine/field_grouping';
import { mergeCascade, seedFromSheet, setPathValue } from '../engine/sheet_defaults';
import type { SheetStore } from '../library/sheet_store';
import { bestEffort } from '../library/sheet_store';

/** The path sets the last successful preview published (WSC-23). */
export interface PreviewMarks {
  /** The flattened paths of the previewed delta — the rows the person changed. */
  readonly modified: ReadonlySet<string>;
  /** The flattened paths of the cascade — the derived rows the server recomputed. */
  readonly recalculated: ReadonlySet<string>;
}

/** The immutable snapshot of one open sheet's edit state (WSC-12 … WSC-25). */
export interface SheetEditState {
  /** The reference values a save advances and `isPending` compares against (WSC-16). */
  readonly defaults: Record<string, unknown>;
  /** The current values; every write replaces the tree rather than mutating it. */
  readonly data: Record<string, unknown>;
  /** Every dotted path written at least once since the seed (WSC-12). */
  readonly editedPaths: ReadonlySet<string>;
  /** A save `PATCH` is in flight (WSC-18). */
  readonly isSaving: boolean;
  /** The mapped copy of the last save failure; cleared on success (WSC-17). */
  readonly saveError: string | undefined;
  /** Increments on every data-changing edit; the preview idempotency key. */
  readonly revision: number;
  /** The revision of the last successful preview (WSC-21, WSC-24). */
  readonly previewedRevision: number | undefined;
  /** The last preview's path sets; absent before a preview, after a save, after an edit (WSC-23, WSC-24). */
  readonly previewMarks: PreviewMarks | undefined;
  /** A preview request is in flight (WSC-21). */
  readonly isPreviewing: boolean;
  /** The mapped copy of the last preview failure; cleared on success (WSC-25). */
  readonly previewError: string | undefined;
}

/** The seed of one edit store: the template's field tree and the stored sheet (WSC-12, WSC-34). */
export interface SheetEditSeed {
  readonly template: TemplateDetail;
  readonly sheet: Sheet;
}

/** The collaborators the store reads through, injected so a suite substitutes fakes (R1). */
export interface SheetEditDeps {
  readonly client: ApiClient;
  readonly store: SheetStore;
}

/** The edit store a sheet screen mounts and every edit suite drives directly. */
export interface SheetEditStore {
  /** The current immutable snapshot. */
  getState(): SheetEditState;
  /** The pending editable delta a save/preview sends and the draft cadence backs up (WSC-14). */
  getPendingDelta(): Record<string, unknown>;
  /** Coarse subscription, for the screen's own signals (footer, marks). */
  subscribe(listener: () => void): () => void;
  /** The per-path subscription a field control uses (AD-001, WSC-34). */
  subscribePath(path: string, listener: () => void): () => void;
  /** Records an edit of one editable leaf (WSC-12). */
  setPath(path: string, value: unknown): void;
  /** Writes one exclusive group's selection in one emission (WSC-15). */
  setExclusive(selected: string | null, siblings: readonly string[]): void;
  /** Sends one delta `PATCH` when `canSave` holds (WSC-14, WSC-16 … WSC-19). */
  save(): Promise<void>;
}

/** True when any edited path's current value differs from its reference (WSC-13). */
export function isPending(state: SheetEditState): boolean {
  for (const path of state.editedPaths) {
    if (!Object.is(readPath(state.data, path), readPath(state.defaults, path))) {
      return true;
    }
  }
  return false;
}

/** True while a save or a preview is in flight; both actions block together (WSC-18, WSC-21). */
export function isBusy(state: SheetEditState): boolean {
  return state.isSaving || state.isPreviewing;
}

/** True when a save may start: pending, not busy, throttle free (WSC-19). */
export function canSave(state: SheetEditState): boolean {
  return isPending(state) && !isBusy(state);
}

/** The editable leaf paths of a field tree, keyed by their dotted path (WSC-14). */
export function editableLeafPaths(fields: Record<string, FieldNode>): Map<string, FieldSchema> {
  const paths = new Map<string, FieldSchema>();
  for (const section of buildSections(fields)) {
    for (const leaf of section.leaves) {
      if (leaf.schema.isEditable) {
        paths.set(leaf.path, leaf.schema);
      }
    }
  }
  return paths;
}

/** Reads a dotted path defensively; a malformed path yields `undefined` rather than raising. */
function readPath(data: Record<string, unknown>, path: string): unknown {
  try {
    return valueAtPath(data, path);
  } catch {
    return undefined;
  }
}

/** Deep-copies a JSON tree so a write never mutates an earlier snapshot (copy-on-write). */
function cloneTree(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(cloneTree);
  }
  if (isPlainObject(value)) {
    const copy: Record<string, unknown> = {};
    for (const [key, child] of Object.entries(value)) {
      copy[key] = cloneTree(child);
    }
    return copy;
  }
  return value;
}

/**
 * Builds the edit store for one open sheet (WSC-12 … WSC-19, WSC-34).
 *
 * The seed overlays the stored sheet on the template's schema defaults and
 * records the template's editable leaves; a later draft overlay and preview land
 * on the same store.
 */
export function createSheetEditStore(seed: SheetEditSeed, deps: SheetEditDeps): SheetEditStore {
  const editable = editableLeafPaths(seed.template.fields);
  const seeded = seedFromSheet(seed.template.fields, seed.sheet.data);

  let state: SheetEditState = {
    defaults: seeded,
    data: seeded,
    editedPaths: new Set<string>(),
    isSaving: false,
    saveError: undefined,
    revision: 0,
    previewedRevision: undefined,
    previewMarks: undefined,
    isPreviewing: false,
    previewError: undefined,
  };

  const listeners = new Set<() => void>();
  const pathListeners = new Map<string, Set<() => void>>();

  const commit = (next: SheetEditState): void => {
    const previous = state;
    state = next;
    for (const [path, pathSubscribers] of pathListeners) {
      if (!Object.is(readPath(previous.data, path), readPath(next.data, path))) {
        for (const listener of [...pathSubscribers]) {
          listener();
        }
      }
    }
    for (const listener of [...listeners]) {
      listener();
    }
  };

  const applyWrites = (writes: ReadonlyMap<string, unknown>): void => {
    const effective = new Map<string, unknown>();
    for (const [path, value] of writes) {
      if (!editable.has(path) || Object.is(readPath(state.data, path), value)) {
        continue;
      }
      effective.set(path, value);
    }
    if (effective.size === 0) {
      return;
    }

    const data = cloneTree(state.data) as Record<string, unknown>;
    const editedPaths = new Set(state.editedPaths);
    for (const [path, value] of effective) {
      setPathValue(data, path, value);
      editedPaths.add(path);
    }

    commit({
      ...state,
      data,
      editedPaths,
      revision: state.revision + 1,
      previewMarks: undefined,
    });
  };

  const pendingDelta = (current: SheetEditState): Record<string, unknown> => {
    const delta: Record<string, unknown> = {};
    for (const path of current.editedPaths) {
      if (
        editable.has(path) &&
        !Object.is(readPath(current.data, path), readPath(current.defaults, path))
      ) {
        setPathValue(delta, path, readPath(current.data, path));
      }
    }
    return delta;
  };

  const hasKeys = (delta: Record<string, unknown>): boolean => Object.keys(delta).length > 0;

  return {
    getState: () => state,
    getPendingDelta: () => pendingDelta(state),

    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },

    subscribePath: (path, listener) => {
      let subscribers = pathListeners.get(path);
      if (subscribers === undefined) {
        subscribers = new Set();
        pathListeners.set(path, subscribers);
      }
      subscribers.add(listener);
      return () => {
        subscribers.delete(listener);
        if (subscribers.size === 0) {
          pathListeners.delete(path);
        }
      };
    },

    setPath: (path, value) => {
      applyWrites(new Map([[path, value]]));
    },

    setExclusive: (selected, siblings) => {
      const writes = new Map<string, unknown>();
      for (const sibling of siblings) {
        writes.set(sibling, sibling === selected ? 1 : 0);
      }
      applyWrites(writes);
    },

    save: async () => {
      if (!canSave(state)) {
        return;
      }
      const delta = pendingDelta(state);
      if (!hasKeys(delta)) {
        return;
      }

      commit({ ...state, isSaving: true, saveError: undefined });

      try {
        const cascade = await deps.client.patchSheet(seed.sheet.id, delta);
        const current = state;
        commit({
          ...current,
          data: mergeCascade(current.data, cascade),
          defaults: mergeCascade(mergeCascade(current.defaults, delta), cascade),
          isSaving: false,
          saveError: undefined,
          previewMarks: undefined,
        });
        await bestEffort(() => deps.store.deleteDraft(seed.sheet.id));
      } catch (error) {
        commit({ ...state, isSaving: false, saveError: failureCopy(error) });
      }
    },
  };
}
