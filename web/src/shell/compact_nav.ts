import { useSyncExternalStore } from 'react';

export const COMPACT_NAV_QUERY = '(max-width: 39.99rem)';

function subscribe(listener: () => void): () => void {
  if (typeof window.matchMedia !== 'function') {
    return () => undefined;
  }
  const query = window.matchMedia(COMPACT_NAV_QUERY);
  query.addEventListener('change', listener);
  return () => query.removeEventListener('change', listener);
}

function read(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(COMPACT_NAV_QUERY).matches;
}

export function useCompactNav(): boolean {
  return useSyncExternalStore(subscribe, read, () => false);
}
