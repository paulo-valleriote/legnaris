type ChangeListener = (event: MediaQueryListEvent) => void;

interface Entry {
  readonly media: string;
  readonly listeners: Set<ChangeListener>;
  readonly mql: MediaQueryList;
}

const entries = new Set<Entry>();
let viewportWidth = 1280;

function evaluate(media: string, width: number): boolean {
  const max = /max-width:\s*(\d+)px/.exec(media);
  if (max) {
    return width <= Number(max[1]);
  }
  const min = /min-width:\s*(\d+)px/.exec(media);
  if (min) {
    return width >= Number(min[1]);
  }
  return false;
}

/** Updates the simulated viewport width and notifies every media-query listener. */
export function setViewportWidth(width: number): void {
  viewportWidth = width;
  for (const entry of entries) {
    const matches = evaluate(entry.media, viewportWidth);
    (entry.mql as { matches: boolean }).matches = matches;
    for (const listener of entry.listeners) {
      listener({ matches, media: entry.media } as MediaQueryListEvent);
    }
  }
}

/** Installs a deterministic `window.matchMedia` for jsdom (which has none). */
export function installMatchMedia(): void {
  viewportWidth = 1280;
  entries.clear();
  window.matchMedia = ((media: string): MediaQueryList => {
    const listeners = new Set<ChangeListener>();
    const mql = {
      media,
      matches: evaluate(media, viewportWidth),
      onchange: null,
      addEventListener: (type: string, listener: ChangeListener) => {
        if (type === 'change') {
          listeners.add(listener);
        }
      },
      removeEventListener: (type: string, listener: ChangeListener) => {
        if (type === 'change') {
          listeners.delete(listener);
        }
      },
      addListener: (listener: ChangeListener) => listeners.add(listener),
      removeListener: (listener: ChangeListener) => listeners.delete(listener),
      dispatchEvent: () => true,
    } as unknown as MediaQueryList;
    entries.add({ media, listeners, mql });
    return mql;
  }) as typeof window.matchMedia;
}
