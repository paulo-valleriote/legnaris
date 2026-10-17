import { useSyncExternalStore } from 'react';
import type { ThemeName } from './tokens';
import type { ThemeStore } from './theme';
import { createThemeStore } from './theme';

let documentStore: ThemeStore | null = null;

/** The document-backed store, created once so every surface reads one source (UIF-18). */
export function getThemeStore(): ThemeStore {
  documentStore ??= createThemeStore({
    storage: window.localStorage,
    matchMedia: (query) => window.matchMedia(query),
    root: document.documentElement,
  });
  return documentStore;
}

export interface ThemeState {
  readonly theme: ThemeName;
  set(theme: ThemeName): void;
  toggle(): void;
}

/** Reads the resolved theme and the two controls, through `useSyncExternalStore` (UIF-18). */
export function useTheme(store: ThemeStore = getThemeStore()): ThemeState {
  const theme = useSyncExternalStore(store.subscribe, store.getSnapshot, store.getSnapshot);
  return { theme, set: store.set, toggle: store.toggle };
}
