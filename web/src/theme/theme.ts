import type { ThemeName } from './tokens';

/** The only persisted theme key; the value is `light` or `dark` (UIF-19). */
export const THEME_STORAGE_KEY = 'lg.theme';

/** The dark color-scheme query the system preference is read from (UIF-17). */
export const DARK_SCHEME_QUERY = '(prefers-color-scheme: dark)';

/** The theme resolved when the system offers no color-scheme signal (UIF-17). */
export const DEFAULT_THEME: ThemeName = 'light';

export interface ThemeStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export interface ThemeStoreDeps {
  readonly storage: ThemeStorage;
  readonly matchMedia: (query: string) => MediaQueryList;
  /** The element whose `data-theme` carries the resolved theme. */
  readonly root: HTMLElement;
}

export interface ThemeStore {
  getSnapshot(): ThemeName;
  subscribe(listener: () => void): () => void;
  set(theme: ThemeName): void;
  toggle(): void;
}

function isTheme(value: unknown): value is ThemeName {
  return value === 'light' || value === 'dark';
}

function readStoredTheme(storage: ThemeStorage): ThemeName | null {
  try {
    const value = storage.getItem(THEME_STORAGE_KEY);
    return isTheme(value) ? value : null;
  } catch {
    return null;
  }
}

function writeStoredTheme(storage: ThemeStorage, theme: ThemeName): void {
  try {
    storage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // A storage that refuses the write must not surface an error (UIF-21).
  }
}

function openMediaQuery(matchMedia: ThemeStoreDeps['matchMedia']): MediaQueryList | null {
  try {
    return matchMedia(DARK_SCHEME_QUERY);
  } catch {
    return null;
  }
}

function watchSystemChange(media: MediaQueryList, handler: () => void): void {
  try {
    media.addEventListener('change', handler);
  } catch {
    // A reader with no event seam is read once, not watched.
  }
}

/**
 * Builds the theme store over injected seams (UIF-17, UIF-19, UIF-23). An explicit stored
 * choice wins from then on; with nothing stored the resolved theme follows the system
 * preference and keeps following it. A storage or reader that throws is swallowed, so the
 * interface never sees a theme error (UIF-21), and an unknown stored value is ignored in
 * favour of the system preference (UIF-22).
 */
export function createThemeStore(deps: ThemeStoreDeps): ThemeStore {
  const listeners = new Set<() => void>();
  const media = openMediaQuery(deps.matchMedia);
  let explicit = readStoredTheme(deps.storage);

  const apply = (theme: ThemeName): void => {
    deps.root.dataset.theme = theme;
  };

  const systemTheme = (): ThemeName => {
    try {
      return media?.matches ? 'dark' : 'light';
    } catch {
      return DEFAULT_THEME;
    }
  };

  let current: ThemeName = explicit ?? systemTheme();
  apply(current);

  const emit = (): void => {
    for (const listener of listeners) {
      listener();
    }
  };

  const followSystem = (): void => {
    if (explicit !== null) {
      return;
    }
    const next = systemTheme();
    if (next === current) {
      return;
    }
    current = next;
    apply(current);
    emit();
  };

  if (media !== null) {
    watchSystemChange(media, followSystem);
  }

  const choose = (theme: ThemeName): void => {
    explicit = theme;
    current = theme;
    apply(current);
    writeStoredTheme(deps.storage, theme);
    emit();
  };

  return {
    getSnapshot: () => current,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    set: choose,
    toggle: () => choose(current === 'dark' ? 'light' : 'dark'),
  };
}
