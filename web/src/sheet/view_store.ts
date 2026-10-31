/**
 * The per-viewer, per-sheet view state of the open sheet: which Sections are
 * collapsed, whether the pinned filter is on and which one-time hints were already taken.
 *
 * It is deliberately not part of the sheet's Layout and not part of the local
 * sheet store: it never reaches the server, never appears in an export and never
 * counts as unsaved work. The collapsed Sections, the filter and the one-time hints are remembered,
 * in `localStorage` behind an injectable seam (like the theme), one record per sheet.
 * Reads and writes that fail are swallowed, and a record that is corrupt, from
 * another version or names a Section the Template no longer has falls back to
 * defaults. Every operation returns the same state reference when it changes
 * nothing, so a subscriber never re-renders for a no-op.
 */

/** The prefix of the one storage key per sheet. */
export const VIEW_STORAGE_PREFIX = 'lg.sheet-view.';

/** The record version a reader accepts. */
export const VIEW_RECORD_VERSION = 1;

/** The slice of `Storage` the view store reads and writes through. */
export interface ViewStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

/** The immutable snapshot of one sheet's view state. */
export interface ViewState {
  /** The collapsed Section keys, in the order they were collapsed; only known Sections. */
  readonly collapsed: readonly string[];
  /** True while only the pinned Rows are shown; remembered with the collapsed Sections. */
  readonly onlyPinned: boolean;
  /** True once the player dismissed the sheet explainer; it then never opens by itself again. */
  readonly explainerSeen: boolean;
}

/** The browser's `localStorage`, or `undefined` where it is missing or refuses access. */
export function browserViewStorage(): ViewStorage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

/** The default view: everything expanded, the filter off. */
export const DEFAULT_VIEW: ViewState = {
  collapsed: [],
  onlyPinned: false,
  explainerSeen: false,
};

/** The storage key of [sheetId]'s view record. */
export function viewStorageKey(sheetId: string): string {
  return `${VIEW_STORAGE_PREFIX}${sheetId}`;
}

function keepKnownUnique(keys: readonly unknown[], known: ReadonlySet<string>): string[] {
  const seen = new Set<string>();
  const kept: string[] = [];
  for (const key of keys) {
    if (typeof key === 'string' && known.has(key) && !seen.has(key)) {
      seen.add(key);
      kept.push(key);
    }
  }
  return kept;
}

/**
 * Reads a stored record into a view. Anything that is not a version-1 object yields the default
 * view. Inside a valid record each field falls back on its own: `collapsed` that is not a list
 * reads as nothing collapsed, keys the Template does not have and repeated keys are dropped, and an
 * `onlyPinned` that is not a boolean (or is absent, as in a record written before the filter was
 * remembered) reads as off.
 */
export function parseViewRecord(raw: string | null, known: ReadonlySet<string>): ViewState {
  if (raw === null) {
    return DEFAULT_VIEW;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return DEFAULT_VIEW;
  }
  if (typeof parsed !== 'object' || parsed === null) {
    return DEFAULT_VIEW;
  }
  const record = parsed as {
    v?: unknown;
    collapsed?: unknown;
    onlyPinned?: unknown;
    explainerSeen?: unknown;
  };
  if (record.v !== VIEW_RECORD_VERSION) {
    return DEFAULT_VIEW;
  }
  return {
    collapsed: Array.isArray(record.collapsed)
      ? keepKnownUnique(record.collapsed as unknown[], known)
      : [],
    onlyPinned: record.onlyPinned === true,
    explainerSeen: record.explainerSeen === true,
  };
}

/**
 * Reads the collapsed Section keys out of a stored record; the same fallbacks as
 * {@link parseViewRecord}.
 */
export function parseCollapsedRecord(raw: string | null, known: ReadonlySet<string>): string[] {
  return [...parseViewRecord(raw, known).collapsed];
}

/** Serializes [state] as the record a reader accepts. */
export function serializeViewRecord(state: ViewState): string {
  return JSON.stringify({
    v: VIEW_RECORD_VERSION,
    collapsed: state.collapsed,
    onlyPinned: state.onlyPinned,
    explainerSeen: state.explainerSeen,
  });
}

/** Collapses [key] when expanded and expands it when collapsed. */
export function toggleCollapsed(state: ViewState, key: string): ViewState {
  const collapsed = state.collapsed.includes(key)
    ? state.collapsed.filter((candidate) => candidate !== key)
    : [...state.collapsed, key];
  return { ...state, collapsed };
}

/** Expands [key]; the same state when it is not collapsed. */
export function expandSection(state: ViewState, key: string): ViewState {
  return state.collapsed.includes(key) ? toggleCollapsed(state, key) : state;
}

/** Collapses every Section in [keys]; the same state when they all already are. */
export function collapseSections(state: ViewState, keys: readonly string[]): ViewState {
  const missing = keys.filter((key) => !state.collapsed.includes(key));
  return missing.length === 0 ? state : { ...state, collapsed: [...state.collapsed, ...missing] };
}

/** Expands every Section; the same state when none is collapsed. */
export function expandAllSections(state: ViewState): ViewState {
  return state.collapsed.length === 0 ? state : { ...state, collapsed: [] };
}

/** Turns the pinned filter on or off; the same state when it already is. */
export function withOnlyPinned(state: ViewState, onlyPinned: boolean): ViewState {
  return state.onlyPinned === onlyPinned ? state : { ...state, onlyPinned };
}

/** Records that the explainer was dismissed; the same state when it already was. */
export function withExplainerSeen(state: ViewState): ViewState {
  return state.explainerSeen ? state : { ...state, explainerSeen: true };
}

/** The view that makes [key] visible: the filter off and the Section expanded. */
export function revealSectionView(state: ViewState, key: string): ViewState {
  return expandSection(withOnlyPinned(state, false), key);
}

/** What a view store is built from: the sheet, its Sections and where it remembers. */
export interface ViewStoreSeed {
  readonly sheetId: string;
  readonly sectionKeys: readonly string[];
  readonly storage: ViewStorage | undefined;
}

/** The store the sheet screen mounts and the view suites drive directly. */
export interface ViewStore {
  getState(): ViewState;
  subscribe(listener: () => void): () => void;
  /** Collapses or expands one Section; a no-op for an unknown key or while the filter is on. */
  toggle(key: string): void;
  /** Collapses every Section; a no-op while the filter is on. */
  collapseAll(): void;
  /** Expands every Section; a no-op while the filter is on. */
  expandAll(): void;
  /** Turns the pinned filter on or off. */
  setOnlyPinned(onlyPinned: boolean): void;
  /** Remembers that the explainer was dismissed. */
  markExplainerSeen(): void;
  /** Remembers that the suggested pins were taken. */
  /** Turns the filter off and expands [key] when it names a Section; an unknown key only turns the filter off. */
  reveal(key: string): void;
}

function readRemembered(seed: ViewStoreSeed): ViewState {
  if (seed.storage === undefined) {
    return DEFAULT_VIEW;
  }
  try {
    return parseViewRecord(
      seed.storage.getItem(viewStorageKey(seed.sheetId)),
      new Set(seed.sectionKeys),
    );
  } catch {
    return DEFAULT_VIEW;
  }
}

/** Builds the store for [seed], reading the remembered view once. */
export function createViewStore(seed: ViewStoreSeed): ViewStore {
  const known = new Set(seed.sectionKeys);
  let state: ViewState = readRemembered(seed);
  const listeners = new Set<() => void>();

  const remember = (): void => {
    try {
      seed.storage?.setItem(viewStorageKey(seed.sheetId), serializeViewRecord(state));
    } catch {
      return;
    }
  };

  const publish = (next: ViewState): void => {
    if (next === state) {
      return;
    }
    state = next;
    remember();
    for (const listener of [...listeners]) {
      listener();
    }
  };

  return {
    getState: () => state,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    toggle: (key) => {
      if (state.onlyPinned || !known.has(key)) {
        return;
      }
      publish(toggleCollapsed(state, key));
    },
    collapseAll: () => {
      if (!state.onlyPinned) {
        publish(collapseSections(state, seed.sectionKeys));
      }
    },
    expandAll: () => {
      if (!state.onlyPinned) {
        publish(expandAllSections(state));
      }
    },
    setOnlyPinned: (onlyPinned) => {
      publish(withOnlyPinned(state, onlyPinned));
    },
    markExplainerSeen: () => {
      publish(withExplainerSeen(state));
    },
    reveal: (key) => {
      publish(known.has(key) ? revealSectionView(state, key) : withOnlyPinned(state, false));
    },
  };
}
