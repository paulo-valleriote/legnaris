/**
 * The in-memory layout store for one open sheet (ADR 0001, ADR 0002).
 *
 * It holds the committed arrangement. Arrange mode, the one way to change it, stages a whole
 * arrangement locally (section order and pins): nothing is sent until `commit()`, which issues
 * exactly one `PUT /sheets/:id/layout`, and `cancel()` drops everything. The store is deliberately separate from the value edit store: a layout change never marks the
 * sheet as having unsaved values and is never part of a value save or preview. A pending
 * arrangement is backed up best-effort through the sheet-store port, on every staged change and
 * without any credential. It owns no React state and binds no listener.
 */

import type { ApiClient } from '../api/client';
import { failureCopy } from '../api/error_mapper';
import type { Layout } from '../api/models/layout';
import type { Arrangement } from '../engine/layout';
import { arrangementToLayout, arrangementsEqual, resolveArrangement } from '../engine/layout';
import { resetArrangement } from '../engine/arrange_ops';
import type { SheetSection } from '../engine/field_grouping';
import type { SheetStore } from '../library/sheet_store';
import { bestEffort } from '../library/sheet_store';

/** The immutable snapshot of one sheet's arrangement state. */
export interface LayoutState {
  /** The arrangement last confirmed by the server (or the Template default). */
  readonly committed: Arrangement;
  /** The arrangement being edited; defined exactly while Arrange mode is on. */
  readonly staged: Arrangement | undefined;
  /** The single `PUT` is in flight. */
  readonly isCommitting: boolean;
  /** The mapped copy of the last commit failure; cleared by the next change, cancel or success. */
  readonly commitError: string | undefined;
}

/** The collaborators the store reads through, injected so a suite substitutes fakes. */
export interface LayoutStoreDeps {
  readonly client: ApiClient;
  readonly store: SheetStore;
  /** Called once a commit succeeds, with the Layout the server stored (`null` after a reset). */
  readonly onCommitted?: (layout: Layout | null) => void;
}

/** What a store is built from: the sheet, its Template's Sections and the committed Layout. */
export interface LayoutStoreSeed {
  readonly sheetId: string;
  readonly sections: readonly SheetSection[];
  readonly layout: Layout | undefined;
}

/** The store a sheet screen mounts and every layout suite drives directly. */
export interface LayoutStore {
  getState(): LayoutState;
  subscribe(listener: () => void): () => void;
  /**
   * Turns Arrange mode on with the committed arrangement staged; a no-op while on or committing.
   */
  enter(): void;
  /**
   * Applies a pure staging operation to the staged arrangement; a no-op outside Arrange mode or
   * while committing. The result is resolved against the Template, so a key naming no Section or
   * Row never stays staged, and a result equal to the current staging changes nothing.
   */
  apply(operation: (staged: Arrangement) => Arrangement): void;
  /** Stages the Template default, to be committed on Done. */
  reset(): void;
  /** Drops the staged arrangement and its backup; a no-op while committing. */
  cancel(): void;
  /** Sends the one `PUT` when the staged arrangement differs from the committed one; otherwise just leaves Arrange mode. */
  commit(): Promise<void>;
  /** Re-enters Arrange mode with the backed-up pending arrangement, when one exists. */
  restore(): Promise<void>;
}

/** True while Arrange mode is on. */
export function isArranging(state: LayoutState): boolean {
  return state.staged !== undefined;
}

/** The arrangement to draw: the staged one, else the committed one. */
export function drawnArrangement(state: LayoutState): Arrangement {
  return state.staged ?? state.committed;
}

/** True when Arrange mode holds a change the server does not have yet. */
export function isLayoutPending(state: LayoutState): boolean {
  return state.staged !== undefined && !arrangementsEqual(state.staged, state.committed);
}

/** Builds the store over the committed Layout of [seed]. */
export function createLayoutStore(seed: LayoutStoreSeed, deps: LayoutStoreDeps): LayoutStore {
  const { sections, sheetId } = seed;
  let state: LayoutState = {
    committed: resolveArrangement(sections, seed.layout),
    staged: undefined,
    isCommitting: false,
    commitError: undefined,
  };
  const listeners = new Set<() => void>();

  const publish = (next: LayoutState): void => {
    state = next;
    for (const listener of [...listeners]) {
      listener();
    }
  };

  const backUp = (next: LayoutState): void => {
    if (next.staged === undefined || !isLayoutPending(next)) {
      void bestEffort(() => deps.store.deleteLayoutDraft(sheetId));
      return;
    }
    const layout = arrangementToLayout(sections, next.staged);
    void bestEffort(() => deps.store.writeLayoutDraft(sheetId, layout));
  };

  const stage = (proposed: Arrangement): void => {
    const current = state.staged;
    const staged = proposed === current ? proposed : resolveArrangement(sections, proposed);
    if (current !== undefined && arrangementsEqual(current, staged)) {
      return;
    }
    const next = { ...state, staged, commitError: undefined };
    publish(next);
    backUp(next);
  };

  return {
    getState: () => state,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    enter: () => {
      if (state.staged !== undefined || state.isCommitting) {
        return;
      }
      publish({ ...state, staged: state.committed, commitError: undefined });
    },
    apply: (operation) => {
      if (state.staged === undefined || state.isCommitting) {
        return;
      }
      stage(operation(state.staged));
    },
    reset: () => {
      if (state.staged === undefined || state.isCommitting) {
        return;
      }
      stage(resetArrangement(sections));
    },
    cancel: () => {
      if (state.isCommitting) {
        return;
      }
      publish({ ...state, staged: undefined, commitError: undefined });
      void bestEffort(() => deps.store.deleteLayoutDraft(sheetId));
    },
    commit: async () => {
      const staged = state.staged;
      if (staged === undefined || state.isCommitting) {
        return;
      }
      if (!isLayoutPending(state)) {
        publish({ ...state, staged: undefined, commitError: undefined });
        void bestEffort(() => deps.store.deleteLayoutDraft(sheetId));
        return;
      }
      publish({ ...state, isCommitting: true, commitError: undefined });
      let stored: Layout | null;
      try {
        stored = await deps.client.setLayout(sheetId, arrangementToLayout(sections, staged));
      } catch (error) {
        publish({ ...state, isCommitting: false, commitError: failureCopy(error) });
        return;
      }
      await bestEffort(() => deps.store.deleteLayoutDraft(sheetId));
      publish({
        ...state,
        committed: resolveArrangement(sections, stored ?? undefined),
        staged: undefined,
        isCommitting: false,
        commitError: undefined,
      });
      await bestEffort(() => Promise.resolve(deps.onCommitted?.(stored)));
    },
    restore: async () => {
      let draft;
      try {
        draft = await deps.store.readLayoutDraft(sheetId);
      } catch {
        return;
      }
      if (draft === null || state.staged !== undefined || state.isCommitting) {
        return;
      }
      const staged = resolveArrangement(sections, draft.layout ?? undefined);
      if (arrangementsEqual(staged, state.committed)) {
        void bestEffort(() => deps.store.deleteLayoutDraft(sheetId));
        return;
      }
      publish({ ...state, staged, commitError: undefined });
    },
  };
}
