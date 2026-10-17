import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { ThemeToggle } from '../shell/theme_toggle';
import type { ThemeStorage } from '../theme/theme';
import { THEME_STORAGE_KEY, createThemeStore } from '../theme/theme';
import { setPrefersDark } from './match_media';

type StorageMode = 'ok' | 'throw-read' | 'throw-write';

interface MemoryStorage {
  readonly storage: ThemeStorage;
  read(key: string): string | undefined;
}

/** A scriptable theme storage: seeded values, plus a read or a write that refuses (UIF-21). */
function memoryStorage(seed: Record<string, string> = {}, mode: StorageMode = 'ok'): MemoryStorage {
  const data = new Map(Object.entries(seed));
  return {
    storage: {
      getItem(key) {
        if (mode === 'throw-read') {
          throw new Error('storage read denied');
        }
        return data.get(key) ?? null;
      },
      setItem(key, value) {
        if (mode === 'throw-write') {
          throw new Error('storage write denied');
        }
        data.set(key, value);
      },
    },
    read: (key) => data.get(key),
  };
}

function browserDeps(storage: ThemeStorage) {
  return {
    storage,
    matchMedia: (query: string) => window.matchMedia(query),
    root: document.createElement('div'),
  };
}

describe('theme resolution (UIF-17, UIF-19, UIF-21, UIF-22, UIF-23)', () => {
  it('Edge: with no stored value the theme follows the system preference', () => {
    setPrefersDark(true);
    const store = createThemeStore(browserDeps(memoryStorage().storage));

    expect(store.getSnapshot()).toBe('dark');
  });

  it('Edge: while nothing is stored, a later system change is followed live', () => {
    setPrefersDark(false);
    const store = createThemeStore(browserDeps(memoryStorage().storage));

    expect(store.getSnapshot()).toBe('light');
    setPrefersDark(true);
    expect(store.getSnapshot()).toBe('dark');
  });

  it('Edge: a stored value outside the accepted set falls back to the system preference', () => {
    setPrefersDark(true);
    const store = createThemeStore(browserDeps(memoryStorage({ [THEME_STORAGE_KEY]: 'purple' }).storage));

    expect(store.getSnapshot()).toBe('dark');
  });

  it('Edge: a storage that throws on read surfaces no error and uses the system preference', () => {
    setPrefersDark(true);
    const refuses = memoryStorage({}, 'throw-read');

    expect(() => createThemeStore(browserDeps(refuses.storage))).not.toThrow();
    expect(createThemeStore(browserDeps(refuses.storage)).getSnapshot()).toBe('dark');
  });

  it('Edge: a storage that throws on write still flips the theme without surfacing an error', () => {
    setPrefersDark(false);
    const store = createThemeStore(browserDeps(memoryStorage({}, 'throw-write').storage));

    expect(() => store.toggle()).not.toThrow();
    expect(store.getSnapshot()).toBe('dark');
  });

  it('Edge: a system change is ignored once an explicit theme is stored', () => {
    setPrefersDark(false);
    const store = createThemeStore(browserDeps(memoryStorage({ [THEME_STORAGE_KEY]: 'light' }).storage));

    setPrefersDark(true);

    expect(store.getSnapshot()).toBe('light');
  });

  it('Edge: with no system signal the theme resolves to the documented default', () => {
    const noSignal = () => ({ matches: false }) as MediaQueryList;
    const store = createThemeStore({ storage: memoryStorage().storage, matchMedia: noSignal, root: document.createElement('div') });

    expect(store.getSnapshot()).toBe('light');
  });
});

describe('theme control (UIF-18, UIF-27, SC-004)', () => {
  it('Happy path: activating the control flips the theme, writes the key and exposes its state', async () => {
    setPrefersDark(false);
    const storage = memoryStorage();
    const store = createThemeStore({
      storage: storage.storage,
      matchMedia: (query) => window.matchMedia(query),
      root: document.documentElement,
    });
    const user = userEvent.setup();
    render(<ThemeToggle store={store} />);

    const button = screen.getByRole('button', { name: message('theme.toggleLabel') });
    expect(button).toHaveAttribute('aria-pressed', 'false');
    expect(button).toHaveAttribute('title', message('theme.switchToDark'));

    await user.click(button);

    expect(button).toHaveAttribute('aria-pressed', 'true');
    expect(button).toHaveAttribute('title', message('theme.switchToLight'));
    expect(storage.read(THEME_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
  });
});
