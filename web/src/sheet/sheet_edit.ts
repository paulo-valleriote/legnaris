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
import { failureCopy, failureKind } from '../api/error_mapper';
import type { FailureKind } from '../api/errors';
import { message } from '../l10n/strings';
import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { FieldNode, FieldSchema, TemplateDetail } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { buildSections } from '../engine/field_grouping';
import { mergeCascade, seedFromSheet, setPathValue } from '../engine/sheet_defaults';
import type { SheetMark } from '../engine/sheet_mark';
import type { SheetStore } from '../library/sheet_store';
import { bestEffort } from '../library/sheet_store';

/** The path sets the last successful preview published (WSC-23). */
export interface PreviewMarks {
  /** The flattened paths of the previewed delta — the rows the person changed. */
  readonly modified: ReadonlySet<string>;
  /** The flattened paths of the cascade — the derived rows the server recomputed. */
  readonly recalculated: ReadonlySet<string>;
}

/** The per-action request cap inside one window (WSC-38, SC-011). */
export const THROTTLE_MAX = 3;

/** The rolling window length, in milliseconds (WSC-38). */
export const THROTTLE_WINDOW_MS = 60_000;

/**
 * The per-action rolling request window (WSC-38, SC-011).
 *
 * A pure, injectable-clock helper: it records the timestamps of the attempts
 * still inside the window, reports whether the cap is reached, and reports when
 * the oldest attempt leaves so an action re-enables without waiting.
 */
export interface RollingWindow {
  /** Records one attempt at the current instant and drops the timestamps that left the window. */
  record(): void;
  /** True while the window holds [THROTTLE_MAX] attempts. */
  isBlocked(): boolean;
  /** The earliest instant a blocked window frees. */
  nextFreeAt(): number;
}

/** Builds a rolling window over the injected [now] clock (WSC-38). */
export function createRollingWindow(
  now: () => number,
  max: number = THROTTLE_MAX,
  windowMs: number = THROTTLE_WINDOW_MS,
): RollingWindow {
  const attempts: number[] = [];

  const prune = (instant: number): void => {
    while (attempts.length > 0 && instant - attempts[0] >= windowMs) {
      attempts.shift();
    }
  };

  return {
    record: () => {
      const instant = now();
      prune(instant);
      attempts.push(instant);
    },
    isBlocked: () => {
      prune(now());
      return attempts.length >= max;
    },
    nextFreeAt: () => {
      prune(now());
      return attempts.length < max ? now() : attempts[0] + windowMs;
    },
  };
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
  /** The kind of the last save failure, so the screen can soften an offline one. */
  readonly saveErrorKind?: FailureKind;
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
  /** The kind of the last preview failure, so the screen can soften an offline one. */
  readonly previewErrorKind?: FailureKind;
  /** The save throttle window (WSC-38). */
  readonly saveWindow: RollingWindow;
  /** The preview throttle window (WSC-38). */
  readonly previewWindow: RollingWindow;
}

/** The seed of one edit store: the template's field tree and the stored sheet's identity and values (WSC-12, WSC-34). */
export interface SheetEditSeed {
  readonly template: TemplateDetail;
  readonly sheet: Pick<Sheet, 'id' | 'data'>;
}

/** The collaborators the store reads through, injected so a suite substitutes fakes (R1). */
export interface SheetEditDeps {
  readonly client: ApiClient;
  readonly store: SheetStore;
  /** The clock the throttle windows read; defaults to `Date.now` (WSC-38, R7). */
  readonly now?: () => number;
  /** The delay seam for the throttle re-evaluation; defaults to `setTimeout` (WSC-38). */
  readonly schedule?: (callback: () => void, delayMs: number) => () => void;
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
  /** A subscription that fires when the preview marks change (WSC-23). */
  subscribeMarks(listener: () => void): () => void;
  /** Records an edit of one editable leaf (WSC-12). */
  setPath(path: string, value: unknown): void;
  /** Writes one exclusive group's selection in one emission (WSC-15). */
  setExclusive(selected: string | null, siblings: readonly string[]): void;
  /** Sends one delta `PATCH` when `canSave` holds (WSC-14, WSC-16 … WSC-19). */
  save(): Promise<void>;
  /** Sends one delta dry-run when `canPreview` holds (WSC-21 … WSC-25). */
  preview(): Promise<void>;
  /** Overlays the stored draft on the seed, marking only its editable leaves pending (WSC-28). */
  restore(): Promise<void>;
  /** Discards every in-memory edit, re-seeding the display from the baseline (WSC-36, WSC-37). */
  discard(): void;
  /** Releases the throttle timer; the screen calls it on unmount (WSC-38). */
  dispose(): void;
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

/** True when a save may start: pending, not busy, its window free (WSC-19, WSC-38). */
export function canSave(state: SheetEditState): boolean {
  return isPending(state) && !isBusy(state) && !state.saveWindow.isBlocked();
}

/** True when a preview may start: pending, a new edit since the last preview, not busy, its window free (WSC-21, WSC-24, WSC-38). */
export function canPreview(state: SheetEditState): boolean {
  return (
    isPending(state) &&
    state.previewedRevision !== state.revision &&
    !isBusy(state) &&
    !state.previewWindow.isBlocked()
  );
}

/**
 * Resolves the mark a row shows from a preview's path sets (WSC-23).
 *
 * A row with any modified path is "modified"; otherwise a row with any
 * recalculated path is "recalculated"; a row the preview never touched carries
 * no mark.
 */
export function markForPaths(
  marks: PreviewMarks | undefined,
  paths: readonly string[],
): SheetMark | undefined {
  if (marks === undefined) {
    return undefined;
  }
  if (paths.some((path) => marks.modified.has(path))) {
    return 'modified';
  }
  if (paths.some((path) => marks.recalculated.has(path))) {
    return 'recalculated';
  }
  return undefined;
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

/** Flattens a nested object into its dotted leaf paths, for the preview marks (WSC-23). */
function flattenPaths(value: Record<string, unknown>, prefix = ''): Set<string> {
  const paths = new Set<string>();
  for (const [key, child] of Object.entries(value)) {
    const path = prefix === '' ? key : `${prefix}.${key}`;
    if (isPlainObject(child)) {
      for (const nested of flattenPaths(child, path)) {
        paths.add(nested);
      }
    } else {
      paths.add(path);
    }
  }
  return paths;
}

/** The default throttle delay: a cancellable `setTimeout` (WSC-38). */
function defaultSchedule(callback: () => void, delayMs: number): () => void {
  const timer = setTimeout(callback, delayMs);
  return () => clearTimeout(timer);
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
  const clock = deps.now ?? Date.now;
  const schedule = deps.schedule ?? defaultSchedule;
  const saveWindow = createRollingWindow(clock);
  const previewWindow = createRollingWindow(clock);

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
    saveWindow,
    previewWindow,
  };

  const listeners = new Set<() => void>();
  const pathListeners = new Map<string, Set<() => void>>();
  const marksListeners = new Set<() => void>();
  let throttleCancel: (() => void) | undefined;

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
    if (previous.previewMarks !== next.previewMarks) {
      for (const listener of [...marksListeners]) {
        listener();
      }
    }
    for (const listener of [...listeners]) {
      listener();
    }
  };

  const scheduleThrottle = (): void => {
    throttleCancel?.();
    throttleCancel = undefined;

    const freed = [
      saveWindow.isBlocked() ? saveWindow.nextFreeAt() : undefined,
      previewWindow.isBlocked() ? previewWindow.nextFreeAt() : undefined,
    ].filter((instant): instant is number => instant !== undefined);
    if (freed.length === 0) {
      return;
    }

    throttleCancel = schedule(
      () => {
        throttleCancel = undefined;
        commit({ ...state });
        scheduleThrottle();
      },
      Math.max(0, Math.min(...freed) - clock()),
    );
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

  /**
   * Resolves the offline copy for a failed request, or `undefined` for any other
   * failure. The pending delta is written to the local draft first, so the
   * "kept on this device" promise holds without waiting for the backup cadence;
   * if that write fails the copy says so instead of promising.
   */
  const offlineCopy = async (
    error: unknown,
    action: 'save' | 'preview',
  ): Promise<string | undefined> => {
    if (failureKind(error) !== 'offline') {
      return undefined;
    }
    const delta = pendingDelta(state);
    let kept = false;
    try {
      await deps.store.writeDraft(seed.sheet.id, delta);
      kept = Object.keys(delta).length > 0;
    } catch {
      kept = false;
    }
    if (!kept) {
      return message(`sheet.error.${action}OfflineUnkept`);
    }
    return message(`sheet.error.${action}Offline`);
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

    subscribeMarks: (listener) => {
      marksListeners.add(listener);
      return () => {
        marksListeners.delete(listener);
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

      saveWindow.record();
      scheduleThrottle();
      commit({ ...state, isSaving: true, saveError: undefined, saveErrorKind: undefined });

      try {
        const cascade = await deps.client.patchSheet(seed.sheet.id, delta);
        const current = state;
        commit({
          ...current,
          data: mergeCascade(current.data, cascade),
          defaults: mergeCascade(mergeCascade(current.defaults, delta), cascade),
          isSaving: false,
          saveError: undefined,
          saveErrorKind: undefined,
          previewMarks: undefined,
        });
        await bestEffort(() => deps.store.deleteDraft(seed.sheet.id));
      } catch (error) {
        const copy = await offlineCopy(error, 'save');
        commit({
          ...state,
          isSaving: false,
          saveError: copy ?? failureCopy(error),
          saveErrorKind: failureKind(error),
        });
      }
    },

    preview: async () => {
      if (!canPreview(state)) {
        return;
      }
      const delta = pendingDelta(state);
      if (!hasKeys(delta)) {
        return;
      }
      const previewedRevision = state.revision;

      previewWindow.record();
      scheduleThrottle();
      commit({
        ...state,
        isPreviewing: true,
        previewError: undefined,
        previewErrorKind: undefined,
      });

      try {
        const cascade = await deps.client.previewSheet(seed.sheet.id, delta);
        const current = state;
        commit({
          ...current,
          data: mergeCascade(current.data, cascade),
          isPreviewing: false,
          previewError: undefined,
          previewErrorKind: undefined,
          previewedRevision,
          previewMarks: {
            modified: flattenPaths(delta),
            recalculated: flattenPaths(cascade),
          },
        });
      } catch (error) {
        const copy = await offlineCopy(error, 'preview');
        commit({
          ...state,
          isPreviewing: false,
          previewError: copy ?? failureCopy(error),
          previewErrorKind: failureKind(error),
        });
      }
    },

    discard: () => {
      commit({
        ...state,
        data: cloneTree(state.defaults) as Record<string, unknown>,
        editedPaths: new Set<string>(),
        revision: state.revision + 1,
        previewedRevision: undefined,
        previewMarks: undefined,
        saveError: undefined,
        saveErrorKind: undefined,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    restore: async () => {
      let draft: Record<string, unknown> | null;
      try {
        draft = await deps.store.readDraft(seed.sheet.id);
      } catch {
        draft = null;
      }
      if (draft === null) {
        return;
      }

      const writes = new Map<string, unknown>();
      for (const path of flattenPaths(draft)) {
        if (editable.has(path)) {
          writes.set(path, readPath(draft, path));
        }
      }
      if (writes.size > 0) {
        applyWrites(writes);
      }
    },

    dispose: () => {
      throttleCancel?.();
      throttleCancel = undefined;
    },
  };
}
