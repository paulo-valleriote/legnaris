import { registerSW } from 'virtual:pwa-register';

/**
 * Owns service-worker registration from an external module (FND-10: no inline
 * registration script) and exposes the pending-update state to the UI (FND-09).
 */

export interface UpdateState {
  readonly needRefresh: boolean;
  readonly offlineReady: boolean;
}

let state: UpdateState = { needRefresh: false, offlineReady: false };
let updateServiceWorker: ((reloadPage?: boolean) => Promise<void>) | undefined;
const listeners = new Set<() => void>();

function emit(next: UpdateState): void {
  state = next;
  for (const listener of listeners) {
    listener();
  }
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function getSnapshot(): UpdateState {
  return state;
}

/** Dismisses the update notice without activating the new worker (FND-09). */
export function dismissUpdate(): void {
  emit({ ...state, needRefresh: false, offlineReady: false });
}

/** Activates the waiting worker and reloads — only called on explicit accept. */
export function applyUpdate(): void {
  void updateServiceWorker?.(true);
}

export function registerServiceWorker(): void {
  updateServiceWorker = registerSW({
    immediate: true,
    onNeedRefresh() {
      emit({ ...state, needRefresh: true });
    },
    onOfflineReady() {
      emit({ ...state, offlineReady: true });
    },
  });
}
