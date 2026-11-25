import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { setPathValue, seedFromSheet, mergeCascade } from '../engine/sheet_defaults';
import type { DiceTarget, SheetEditState, SheetEditStore } from '../sheet/sheet_edit';
import {
  canPreview,
  canSave,
  createRollingWindow,
  editableLeafPaths,
  isDicePending,
} from '../sheet/sheet_edit';

const EMPTY_DICE = { section: {}, row: {} };

function readPath(data: Record<string, unknown>, path: string): unknown {
  try {
    return valueAtPath(data, path);
  } catch {
    return undefined;
  }
}

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

export type DemoDerivation = (data: Record<string, unknown>) => ReadonlyMap<string, number>;

function applyDerived(data: Record<string, unknown>, derive: DemoDerivation): void {
  for (const [path, value] of derive(data)) {
    setPathValue(data, path, value);
  }
}

function withDerived(
  seed: Record<string, unknown>,
  derive: DemoDerivation,
): Record<string, unknown> {
  const derived = cloneTree(seed) as Record<string, unknown>;
  applyDerived(derived, derive);
  return derived;
}

export function createDemoEditStore(
  template: TemplateDetail,
  sheet: Sheet,
  derive: DemoDerivation,
): SheetEditStore {
  const editable = editableLeafPaths(template.fields);
  const seeded = withDerived(seedFromSheet(template.fields, sheet.data), derive);
  const seededDice = sheet.dice ?? EMPTY_DICE;
  const saveWindow = createRollingWindow(() => Date.now(), Number.POSITIVE_INFINITY);
  const previewWindow = createRollingWindow(() => Date.now(), Number.POSITIVE_INFINITY);

  let state: SheetEditState = {
    defaults: seeded,
    data: cloneTree(seeded) as Record<string, unknown>,
    editedPaths: new Set<string>(),
    savedDice: seededDice,
    dice: seededDice,
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
      if (Object.is(readPath(data, path), readPath(state.defaults, path))) {
        editedPaths.delete(path);
      } else {
        editedPaths.add(path);
      }
    }
    applyDerived(data, derive);

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

  const pendingDice = (current: SheetEditState) =>
    isDicePending(current) ? current.dice : undefined;

  const withDiceEntry = (
    overrides: Readonly<Record<string, string | null>>,
    key: string,
    expression: string | null | undefined,
  ): Record<string, string | null> => {
    const next = { ...overrides };
    if (expression === undefined) {
      delete next[key];
    } else {
      next[key] = expression;
    }
    return next;
  };

  const sameDiceEntry = (
    overrides: Readonly<Record<string, string | null>>,
    key: string,
    expression: string | null | undefined,
  ): boolean =>
    expression === undefined
      ? !Object.hasOwn(overrides, key)
      : Object.hasOwn(overrides, key) && overrides[key] === expression;

  return {
    getState: () => state,
    getPendingDelta: () => pendingDelta(state),
    getPendingDice: () => pendingDice(state),

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

    applyReset: (cascade) => {
      const data = mergeCascade(state.data, cascade);
      applyDerived(data, derive);
      commit({
        ...state,
        data,
        defaults: mergeCascade(state.defaults, cascade),
        revision: state.revision + 1,
        previewedRevision: undefined,
        previewMarks: undefined,
      });
    },

    refresh: () => Promise.resolve(),

    setDice: (target: DiceTarget, key, expression) => {
      const current = target === 'row' ? state.dice.row : state.dice.section;
      if (sameDiceEntry(current, key, expression)) {
        return;
      }
      const next = withDiceEntry(current, key, expression);
      commit({
        ...state,
        dice: target === 'row' ? { ...state.dice, row: next } : { ...state.dice, section: next },
      });
    },

    save: async () => {
      if (!canSave(state)) {
        return;
      }
      saveWindow.record();
      commit({
        ...state,
        defaults: cloneTree(state.data) as Record<string, unknown>,
        editedPaths: new Set<string>(),
        savedDice: state.dice,
        isSaving: false,
        saveError: undefined,
        saveErrorKind: undefined,
        previewMarks: undefined,
      });
    },

    preview: async () => {
      if (!canPreview(state)) {
        return;
      }
      previewWindow.record();
      commit({
        ...state,
        isPreviewing: false,
        previewedRevision: state.revision,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    restore: async () => undefined,

    discard: () => {
      commit({
        ...state,
        data: cloneTree(state.defaults) as Record<string, unknown>,
        editedPaths: new Set<string>(),
        dice: state.savedDice,
        revision: state.revision + 1,
        previewedRevision: undefined,
        previewMarks: undefined,
        saveError: undefined,
        saveErrorKind: undefined,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    dispose: () => undefined,
  };
}
