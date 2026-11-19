type ChangeListener = (event: MediaQueryListEvent) => void;

interface Entry {
  readonly media: string;
  readonly listeners: Set<ChangeListener>;
  readonly mql: MediaQueryList;
}

const entries = new Set<Entry>();
let viewportWidth = 1280;
let prefersDark = false;

function lengthToPx(value: string, unit: string): number {
  const size = Number(value);
  return unit === 'rem' || unit === 'em' ? size * 16 : size;
}

function evaluate(media: string, width: number): boolean {
  const max = /max-width:\s*([\d.]+)(px|rem|em)/.exec(media);
  if (max) {
    return width <= lengthToPx(max[1], max[2]);
  }
  const min = /min-width:\s*([\d.]+)(px|rem|em)/.exec(media);
  if (min) {
    return width >= lengthToPx(min[1], min[2]);
  }
  if (/prefers-color-scheme:\s*dark/.test(media)) {
    return prefersDark;
  }
  if (/prefers-color-scheme:\s*light/.test(media)) {
    return !prefersDark;
  }
  return false;
}

function notify(): void {
  for (const entry of entries) {
    const matches = evaluate(entry.media, viewportWidth);
    (entry.mql as { matches: boolean }).matches = matches;
    for (const listener of entry.listeners) {
      listener({ matches, media: entry.media } as MediaQueryListEvent);
    }
  }
}

export function setViewportWidth(width: number): void {
  viewportWidth = width;
  notify();
}

export function setPrefersDark(value: boolean): void {
  prefersDark = value;
  notify();
}

export function installMatchMedia(): void {
  viewportWidth = 1280;
  prefersDark = false;
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
