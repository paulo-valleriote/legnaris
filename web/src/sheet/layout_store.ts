import type { ApiClient } from '../api/client';
import { failureCopy } from '../api/error_mapper';
import type { Layout } from '../api/models/layout';
import type { Arrangement } from '../engine/layout';
import { arrangementToLayout, arrangementsEqual, resolveArrangement } from '../engine/layout';
import { resetArrangement } from '../engine/arrange_ops';
import type { SheetSection } from '../engine/field_grouping';
import type { SheetStore } from '../library/sheet_store';
import { bestEffort } from '../library/sheet_store';

export interface LayoutState {
  readonly committed: Arrangement;
  readonly staged: Arrangement | undefined;
  readonly isCommitting: boolean;
  readonly commitError: string | undefined;
}

export interface LayoutStoreDeps {
  readonly client: ApiClient;
  readonly store: SheetStore;
  readonly onCommitted?: (layout: Layout | null) => void;
}

export interface LayoutStoreSeed {
  readonly sheetId: string;
  readonly sections: readonly SheetSection[];
  readonly layout: Layout | undefined;
}

export interface LayoutStore {
  getState(): LayoutState;
  subscribe(listener: () => void): () => void;
  enter(): void;
  apply(operation: (staged: Arrangement) => Arrangement): void;
  reset(): void;
  cancel(): void;
  commit(): Promise<void>;
  restore(): Promise<void>;
}

export function isArranging(state: LayoutState): boolean {
  return state.staged !== undefined;
}

export function drawnArrangement(state: LayoutState): Arrangement {
  return state.staged ?? state.committed;
}

export function isLayoutPending(state: LayoutState): boolean {
  return state.staged !== undefined && !arrangementsEqual(state.staged, state.committed);
}

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
