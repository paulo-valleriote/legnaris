import type { ApiClient } from '../api/client';
import { failureCopy } from '../api/error_mapper';
import { DomainFailure } from '../api/errors';
import type { Sheet } from '../api/models/sheet';
import { evictSheet } from '../library/sheet_library';
import type { SheetStore } from '../library/sheet_store';

export type YourSheetState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'empty' }
  | { readonly kind: 'sheet'; readonly sheet: Sheet }
  | { readonly kind: 'failure'; readonly message: string };

export interface YourSheetSnapshot {
  readonly state: YourSheetState;
  readonly deleting: boolean;
  readonly deleteError: string | undefined;
}

export interface YourSheetController {
  getSnapshot(): YourSheetSnapshot;
  subscribe(listener: () => void): () => void;
  resolve(): Promise<void>;
  remove(id: string): Promise<void>;
}

export interface YourSheetOptions {
  readonly client: Pick<ApiClient, 'getMySheet' | 'deleteSheet'>;
  readonly store: SheetStore;
}

export function createYourSheet({ client, store }: YourSheetOptions): YourSheetController {
  let state: YourSheetState = { kind: 'loading' };
  let deleting = false;
  let deleteError: string | undefined;
  const listeners = new Set<() => void>();
  let snapshot: YourSheetSnapshot = Object.freeze({ state, deleting, deleteError });

  function emit(): void {
    snapshot = Object.freeze({ state, deleting, deleteError });
    for (const listener of listeners) {
      listener();
    }
  }

  async function resolve(): Promise<void> {
    state = { kind: 'loading' };
    deleteError = undefined;
    emit();

    try {
      const sheet = await client.getMySheet();
      state = { kind: 'sheet', sheet };
      emit();
    } catch (error) {
      state =
        error instanceof DomainFailure && error.kind === 'notFound'
          ? { kind: 'empty' }
          : { kind: 'failure', message: failureCopy(error) };
      emit();
    }
  }

  async function remove(id: string): Promise<void> {
    if (deleting) {
      return;
    }

    deleting = true;
    deleteError = undefined;
    emit();

    try {
      await client.deleteSheet(id);
      await evictSheet(store, id);
      deleting = false;
      state = { kind: 'empty' };
      emit();
    } catch (error) {
      deleting = false;
      deleteError = failureCopy(error);
      emit();
    }
  }

  return {
    getSnapshot: () => snapshot,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    resolve,
    remove,
  };
}
