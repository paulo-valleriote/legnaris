/**
 * The "Your sheet" destination state machine (WAS-25 … WAS-29, R10).
 *
 * The server is the source of truth: the destination resolves the caller's
 * single owner-scoped sheet through one `GET /sheets`, mapping a `NOT_FOUND` to
 * the designed empty state and any other outcome — an undecodable payload
 * included — to a mapped failure with a single-request Retry. On confirm the
 * delete issues exactly one `DELETE /sheets/:id` and, on success, evicts the
 * sheet and its draft locally before showing the empty state; on failure the
 * sheet is kept and the mapped copy is shown.
 */

import type { ApiClient } from '../api/client';
import { failureCopy } from '../api/error_mapper';
import { DomainFailure } from '../api/errors';
import type { Sheet } from '../api/models/sheet';
import { evictSheet } from '../library/sheet_library';
import type { SheetStore } from '../library/sheet_store';

/** The one resolve situation the destination renders (WAS-25 … WAS-27). */
export type YourSheetState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'empty' }
  | { readonly kind: 'sheet'; readonly sheet: Sheet }
  | { readonly kind: 'failure'; readonly message: string };

/** The snapshot a subscriber reads (R10). */
export interface YourSheetSnapshot {
  readonly state: YourSheetState;
  readonly deleting: boolean;
  readonly deleteError: string | undefined;
}

/** The resolve/delete controller the screen subscribes to (R10). */
export interface YourSheetController {
  getSnapshot(): YourSheetSnapshot;
  subscribe(listener: () => void): () => void;
  /** Re-resolves the caller's sheet with one `GET /sheets` (WAS-25, WAS-27). */
  resolve(): Promise<void>;
  /** Confirms the delete: one `DELETE /sheets/:id`, then evicts locally (WAS-28, WAS-29). */
  remove(id: string): Promise<void>;
}

/** Construction options for {@link createYourSheet}. */
export interface YourSheetOptions {
  readonly client: Pick<ApiClient, 'getMySheet' | 'deleteSheet'>;
  readonly store: SheetStore;
}

/** Builds the destination controller over the client and the local store (R10). */
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
