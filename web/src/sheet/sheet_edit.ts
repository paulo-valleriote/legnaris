import type { ApiClient } from '../api/client';
import type { DiceBlock, DiceOverrides } from '../api/models/dice';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { SaveRetryDeps } from './save_retry';
import { isVersionConflict, withConflictRetry } from './save_retry';
import type { FailureKind } from '../api/errors';
import { message } from '../l10n/strings';
import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { FieldNode, FieldSchema, TemplateDetail } from '../api/models/template';
import { hasEntries, isPlainObject } from '../api/parse';
import { buildSections } from '../engine/field_grouping';
import {
  isRecordPositionPath,
  isRowKey as isTemplateRowKey,
  listInfoAtPath,
  newRecordID,
  orderedRecords,
  parseRecordRowKey,
  recordFieldSchema,
  recordRowKey,
} from '../engine/row_identity';
import {
  diceSchemaFromFields,
  diceSectionRows,
  pruneStaleChoices,
  type PruneContext,
} from './dice_view';
import { mergeCascade, seedFromSheet, seedValue, setPathValue } from '../engine/sheet_defaults';
import type { SheetMark } from '../engine/sheet_mark';
import type { SheetDraft, SheetStore } from '../library/sheet_store';
import { bestEffort, hasPendingWork } from '../library/sheet_store';

export interface PreviewMarks {
  readonly modified: ReadonlySet<string>;
  readonly recalculated: ReadonlySet<string>;
}

export const THROTTLE_MAX = 3;

export const THROTTLE_WINDOW_MS = 60_000;

export interface RollingWindow {
  record(): void;
  isBlocked(): boolean;
  nextFreeAt(): number;
}

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

export interface SheetEditState {
  readonly defaults: Record<string, unknown>;
  readonly data: Record<string, unknown>;
  readonly editedPaths: ReadonlySet<string>;
  readonly isSaving: boolean;
  readonly saveError: string | undefined;
  readonly saveErrorKind?: FailureKind;
  readonly revision: number;
  readonly previewedRevision: number | undefined;
  readonly savedDice: DiceBlock;
  readonly dice: DiceBlock;
  readonly previewMarks: PreviewMarks | undefined;
  readonly isPreviewing: boolean;
  readonly previewError: string | undefined;
  readonly previewErrorKind?: FailureKind;
  readonly saveWindow: RollingWindow;
  readonly previewWindow: RollingWindow;
}

export interface SheetEditSeed {
  readonly template: TemplateDetail;
  readonly sheet: Pick<Sheet, 'id' | 'data' | 'dice'>;
}

export interface SheetEditDeps {
  readonly client: ApiClient;
  readonly store: SheetStore;
  readonly now?: () => number;
  readonly schedule?: (callback: () => void, delayMs: number) => () => void;
  readonly saveRetry?: SaveRetryDeps;
}

export interface SheetEditStore {
  getState(): SheetEditState;
  getPendingDelta(): Record<string, unknown>;
  getPendingDice(): DiceBlock | undefined;
  subscribe(listener: () => void): () => void;
  subscribePath(path: string, listener: () => void): () => void;
  subscribeMarks(listener: () => void): () => void;
  setPath(path: string, value: unknown): void;
  setExclusive(selected: string | null, siblings: readonly string[]): void;
  setDice(target: DiceTarget, key: string, expression: string | null | undefined): void;
  addRecord?(listPath: string): string | undefined;
  removeRecord?(listPath: string, recordID: string): void;
  moveRecord?(listPath: string, recordID: string, direction: -1 | 1): number | undefined;
  applyReset(cascade: Record<string, unknown>): void;
  refresh(): Promise<void>;
  save(): Promise<void>;
  preview(): Promise<void>;
  restore(): Promise<void>;
  discard(): void;
  dispose(): void;
}

export type RecordEditStore = SheetEditStore &
  Required<Pick<SheetEditStore, 'addRecord' | 'removeRecord' | 'moveRecord'>>;

export type DiceTarget = 'row' | 'section';

export interface PendingDiceKeys {
  readonly row: ReadonlySet<string>;
  readonly section: ReadonlySet<string>;
}

function differingKeys(current: DiceOverrides, saved: DiceOverrides): Set<string> {
  const keys = new Set<string>();
  for (const key of new Set([...Object.keys(current), ...Object.keys(saved)])) {
    if (Object.hasOwn(current, key) !== Object.hasOwn(saved, key) || current[key] !== saved[key]) {
      keys.add(key);
    }
  }
  return keys;
}

export function pendingDiceKeys(state: SheetEditState): PendingDiceKeys {
  return {
    row: differingKeys(state.dice.row, state.savedDice.row),
    section: differingKeys(state.dice.section, state.savedDice.section),
  };
}

export function isDicePending(state: SheetEditState): boolean {
  const keys = pendingDiceKeys(state);
  return keys.row.size > 0 || keys.section.size > 0;
}

function valuesPendingCount(state: SheetEditState): number {
  let count = 0;
  for (const path of state.editedPaths) {
    if (!Object.is(readPath(state.data, path), readPath(state.defaults, path))) {
      count += 1;
    }
  }
  return count;
}

export function areValuesPending(state: SheetEditState): boolean {
  return valuesPendingCount(state) > 0;
}

export function pendingCount(state: SheetEditState): number {
  const keys = pendingDiceKeys(state);
  return valuesPendingCount(state) + keys.row.size + keys.section.size;
}

export function isPending(state: SheetEditState): boolean {
  return areValuesPending(state) || isDicePending(state);
}

export function isBusy(state: SheetEditState): boolean {
  return state.isSaving || state.isPreviewing;
}

export function canSave(state: SheetEditState): boolean {
  return isPending(state) && !isBusy(state) && !state.saveWindow.isBlocked();
}

export function canPreview(state: SheetEditState): boolean {
  return (
    areValuesPending(state) &&
    state.previewedRevision !== state.revision &&
    !isBusy(state) &&
    !state.previewWindow.isBlocked()
  );
}

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

const EMPTY_DICE: DiceBlock = { section: {}, row: {} };

function withDiceEntry(
  overrides: DiceOverrides,
  key: string,
  expression: string | null | undefined,
): DiceOverrides {
  const next: Record<string, string | null> = { ...overrides };
  if (expression === undefined) {
    delete next[key];
  } else {
    next[key] = expression;
  }
  return next;
}

function sameDiceEntry(
  overrides: DiceOverrides,
  key: string,
  expression: string | null | undefined,
): boolean {
  if (expression === undefined) {
    return !Object.hasOwn(overrides, key);
  }
  return Object.hasOwn(overrides, key) && overrides[key] === expression;
}

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

function defaultSchedule(callback: () => void, delayMs: number): () => void {
  const timer = setTimeout(callback, delayMs);
  return () => clearTimeout(timer);
}

export function createSheetEditStore(seed: SheetEditSeed, deps: SheetEditDeps): RecordEditStore {
  const editable = editableLeafPaths(seed.template.fields);
  const isWritablePath = (path: string): boolean => {
    if (editable.has(path)) {
      return true;
    }
    if (recordFieldSchema(seed.template.fields, path)?.isEditable === true) {
      return true;
    }
    if (parseRecordRowKey(seed.template.fields, path) !== undefined) {
      return true;
    }
    return isRecordPositionPath(seed.template.fields, path);
  };
  const sectionRows = diceSectionRows(seed.template.fields);
  const diceRules: PruneContext = {
    rowKeys: new Set([...sectionRows.values()].flat()),
    sectionRows,
    schema: diceSchemaFromFields(seed.template.fields),
    isRowKey: (key) => isTemplateRowKey(seed.template.fields, key),
  };
  const seededDice = pruneStaleChoices(seed.sheet.dice ?? EMPTY_DICE, diceRules);
  const seeded = seedFromSheet(seed.template.fields, seed.sheet.data);
  const clock = deps.now ?? Date.now;
  const schedule = deps.schedule ?? defaultSchedule;
  const saveWindow = createRollingWindow(clock);
  const previewWindow = createRollingWindow(clock);

  let state: SheetEditState = {
    defaults: seeded,
    data: seeded,
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
      if (!isWritablePath(path) || Object.is(readPath(state.data, path), value)) {
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

  const pendingDice = (current: SheetEditState): DiceBlock | undefined =>
    isDicePending(current) ? current.dice : undefined;

  const pendingDelta = (current: SheetEditState): Record<string, unknown> => {
    const delta: Record<string, unknown> = {};
    for (const path of current.editedPaths) {
      if (
        isWritablePath(path) &&
        !Object.is(readPath(current.data, path), readPath(current.defaults, path))
      ) {
        setPathValue(delta, path, readPath(current.data, path));
      }
    }
    return delta;
  };

  const offlineCopy = async (
    error: unknown,
    action: 'save' | 'preview',
  ): Promise<string | undefined> => {
    if (failureKind(error) !== 'offline') {
      return undefined;
    }
    const delta = pendingDelta(state);
    const dice = pendingDice(state);
    let kept = false;
    try {
      await deps.store.writeDraft(seed.sheet.id, delta, dice);
      kept = hasPendingWork({ delta, dice });
    } catch {
      kept = false;
    }
    if (!kept) {
      return message(`sheet.error.${action}OfflineUnkept`);
    }
    return message(`sheet.error.${action}Offline`);
  };

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

    setDice: (target, key, expression) => {
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

    addRecord: (listPath) => {
      const list = listInfoAtPath(seed.template.fields, listPath);
      if (list === undefined) {
        return undefined;
      }
      const existing = orderedRecords(listPath, state.data);
      if (existing.length >= list.capacity) {
        return undefined;
      }
      const recordID = newRecordID();
      const nextPosition =
        existing.reduce(
          (highest, entry) =>
            Number.isFinite(entry.position) ? Math.max(highest, entry.position) : highest,
          0,
        ) + 1;
      const writes = new Map<string, unknown>();
      writes.set(`${listPath}.${recordID}.position`, nextPosition);
      for (const [name, schema] of Object.entries(list.shape)) {
        if (schema.isEditable) {
          writes.set(`${listPath}.${recordID}.${name}`, seedValue(schema));
        }
      }
      applyWrites(writes);
      return recordID;
    },

    removeRecord: (listPath, recordID) => {
      const exists = orderedRecords(listPath, state.data).some((entry) => entry.id === recordID);
      if (!exists) {
        return;
      }
      applyWrites(new Map([[recordRowKey(listPath, recordID), null]]));
    },

    moveRecord: (listPath, recordID, direction) => {
      const order = orderedRecords(listPath, state.data).map((entry) => entry.id);
      const from = order.indexOf(recordID);
      const to = from + direction;
      if (from < 0 || to < 0 || to >= order.length) {
        return undefined;
      }
      const next = [...order];
      next[from] = order[to];
      next[to] = order[from];
      const writes = new Map<string, unknown>();
      next.forEach((id, index) => {
        writes.set(`${listPath}.${id}.position`, index + 1);
      });
      applyWrites(writes);
      return to + 1;
    },

    applyReset: (cascade) => {
      commit({
        ...state,
        data: mergeCascade(state.data, cascade),
        defaults: mergeCascade(state.defaults, cascade),
        revision: state.revision + 1,
        previewedRevision: undefined,
        previewMarks: undefined,
        saveError: undefined,
        saveErrorKind: undefined,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    refresh: async () => {
      if (isPending(state) || isBusy(state)) {
        return;
      }
      const fresh = await deps.client.getSheet(seed.sheet.id);
      const seeded = seedFromSheet(seed.template.fields, fresh.data);
      const dice = pruneStaleChoices(fresh.dice ?? EMPTY_DICE, diceRules);
      commit({
        ...state,
        defaults: seeded,
        data: seeded,
        editedPaths: new Set<string>(),
        savedDice: dice,
        dice,
        revision: state.revision + 1,
        previewedRevision: undefined,
        previewMarks: undefined,
        saveError: undefined,
        saveErrorKind: undefined,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    save: async () => {
      if (!canSave(state)) {
        return;
      }
      const delta = pendingDelta(state);
      const dice = pendingDice(state);
      if (!hasPendingWork({ delta, dice })) {
        return;
      }

      saveWindow.record();
      scheduleThrottle();
      commit({ ...state, isSaving: true, saveError: undefined, saveErrorKind: undefined });

      try {
        const cascade = await withConflictRetry(
          () => deps.client.patchSheet(seed.sheet.id, delta, dice),
          deps.saveRetry,
        );
        const current = state;
        commit({
          ...current,
          data: mergeCascade(current.data, cascade),
          defaults: mergeCascade(mergeCascade(current.defaults, delta), cascade),
          savedDice: dice ?? current.savedDice,
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
          saveError:
            copy ??
            (isVersionConflict(error) ? message('sheet.error.saveConflict') : failureCopy(error)),
          saveErrorKind: failureKind(error),
        });
      }
    },

    preview: async () => {
      if (!canPreview(state)) {
        return;
      }
      const delta = pendingDelta(state);
      if (!hasEntries(delta)) {
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

    restore: async () => {
      let draft: SheetDraft | null;
      try {
        draft = await deps.store.readDraft(seed.sheet.id);
      } catch {
        draft = null;
      }
      if (draft === null) {
        return;
      }

      const writes = new Map<string, unknown>();
      for (const path of flattenPaths(draft.delta)) {
        if (isWritablePath(path)) {
          writes.set(path, readPath(draft.delta, path));
        }
      }
      if (writes.size > 0) {
        applyWrites(writes);
      }
      if (draft.dice !== undefined) {
        commit({
          ...state,
          dice: pruneStaleChoices(draft.dice, diceRules),
        });
      }
    },

    dispose: () => {
      throttleCancel?.();
      throttleCancel = undefined;
    },
  };
}
