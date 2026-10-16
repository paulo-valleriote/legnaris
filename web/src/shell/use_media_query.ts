import { useCallback, useSyncExternalStore } from 'react';

/**
 * Subscribes to a CSS media query. The shell uses this so the collapse
 * behaviour is observable in tests without a real layout engine (FND-03).
 */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onStoreChange: () => void) => {
      const list = window.matchMedia(query);
      list.addEventListener('change', onStoreChange);
      return () => list.removeEventListener('change', onStoreChange);
    },
    [query],
  );

  const getSnapshot = useCallback(() => window.matchMedia(query).matches, [query]);

  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
