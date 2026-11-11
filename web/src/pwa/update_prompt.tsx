import { useSyncExternalStore } from 'react';
import { useAuthSession } from '../app/providers';
import { applyUpdate, dismissUpdate, getSnapshot, subscribe } from './register_sw';
import './update_prompt.css';

export interface UpdatePromptViewProps {
  readonly needRefresh: boolean;
  readonly offlineReady: boolean;
  readonly onUpdate: () => void;
  readonly onDismiss: () => void;
}

/**
 * Non-blocking update / offline-ready notice (FND-09). Presentational, so its
 * behaviour is testable without a browser worker.
 */
export function UpdatePromptView({
  needRefresh,
  offlineReady,
  onUpdate,
  onDismiss,
}: UpdatePromptViewProps) {
  if (!needRefresh && !offlineReady) {
    return null;
  }
  return (
    <div className="lg-update" role="status" aria-live="polite" data-testid="update-prompt">
      <p className="lg-update__text">
        {needRefresh ? 'A new version is available.' : 'Ready to work offline.'}
      </p>
      <div className="lg-update__actions">
        {needRefresh ? (
          <button type="button" className="lg-update__button" onClick={onUpdate}>
            Reload
          </button>
        ) : null}
        <button type="button" className="lg-update__button" onClick={onDismiss}>
          Dismiss
        </button>
      </div>
    </div>
  );
}

/**
 * Wires the presentational notice to the service-worker registration store. The offline-ready
 * notice waits for a session so it never competes with sign-in.
 */
export function UpdatePrompt() {
  const state = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  const session = useAuthSession();
  const auth = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const signedIn = auth.state === 'authenticated';
  return (
    <UpdatePromptView
      needRefresh={state.needRefresh}
      offlineReady={state.offlineReady && signedIn}
      onUpdate={applyUpdate}
      onDismiss={dismissUpdate}
    />
  );
}
