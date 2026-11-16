export const VIEW_STORAGE_PREFIX = 'lg.sheet-view.';

export const VIEW_RECORD_VERSION = 1;

export interface ViewStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export interface ViewState {
  readonly collapsed: readonly string[];
  readonly onlyPinned: boolean;
  readonly explainerSeen: boolean;
  readonly settingsOpen: boolean;
}

export function browserViewStorage(): ViewStorage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

export const DEFAULT_VIEW: ViewState = {
  collapsed: [],
  onlyPinned: false,
  explainerSeen: false,
  settingsOpen: false,
};

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
    settingsOpen?: unknown;
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
    settingsOpen: record.settingsOpen === true,
  };
}

export function parseCollapsedRecord(raw: string | null, known: ReadonlySet<string>): string[] {
  return [...parseViewRecord(raw, known).collapsed];
}

export function serializeViewRecord(state: ViewState): string {
  return JSON.stringify({
    v: VIEW_RECORD_VERSION,
    collapsed: state.collapsed,
    onlyPinned: state.onlyPinned,
    explainerSeen: state.explainerSeen,
    settingsOpen: state.settingsOpen,
  });
}

export function toggleCollapsed(state: ViewState, key: string): ViewState {
  const collapsed = state.collapsed.includes(key)
    ? state.collapsed.filter((candidate) => candidate !== key)
    : [...state.collapsed, key];
  return { ...state, collapsed };
}

export function expandSection(state: ViewState, key: string): ViewState {
  return state.collapsed.includes(key) ? toggleCollapsed(state, key) : state;
}

export function collapseSections(state: ViewState, keys: readonly string[]): ViewState {
  const missing = keys.filter((key) => !state.collapsed.includes(key));
  return missing.length === 0 ? state : { ...state, collapsed: [...state.collapsed, ...missing] };
}

export function expandAllSections(state: ViewState): ViewState {
  return state.collapsed.length === 0 ? state : { ...state, collapsed: [] };
}

export function withOnlyPinned(state: ViewState, onlyPinned: boolean): ViewState {
  return state.onlyPinned === onlyPinned ? state : { ...state, onlyPinned };
}

export function withExplainerSeen(state: ViewState): ViewState {
  return state.explainerSeen ? state : { ...state, explainerSeen: true };
}

export function toggleSettings(state: ViewState): ViewState {
  return { ...state, settingsOpen: !state.settingsOpen };
}

export function revealSectionView(state: ViewState, key: string): ViewState {
  const shown = withOnlyPinned(state, false);
  if (TEMPLATE_SETTINGS_KEYS.includes(key)) {
    return shown.settingsOpen ? shown : { ...shown, settingsOpen: true };
  }
  return expandSection(shown, key);
}

export interface ViewStoreSeed {
  readonly sheetId: string;
  readonly sectionKeys: readonly string[];
  readonly storage: ViewStorage | undefined;
}

export interface ViewStore {
  getState(): ViewState;
  subscribe(listener: () => void): () => void;
  toggle(key: string): void;
  collapseAll(): void;
  expandAll(): void;
  setOnlyPinned(onlyPinned: boolean): void;
  markExplainerSeen(): void;
  toggleSettings(): void;
  reveal(key: string): void;
}

export const TEMPLATE_SETTINGS_KEYS: readonly string[] = ['rules', 'config'];

export function isTemplateSettingsKey(key: string): boolean {
  return TEMPLATE_SETTINGS_KEYS.includes(key);
}

export function mainSectionKeys(sectionKeys: readonly string[]): string[] {
  return sectionKeys.filter((key) => !isTemplateSettingsKey(key));
}

export function initialView(): ViewState {
  return DEFAULT_VIEW;
}

function readRemembered(seed: ViewStoreSeed): ViewState {
  if (seed.storage === undefined) {
    return initialView();
  }
  try {
    const raw = seed.storage.getItem(viewStorageKey(seed.sheetId));
    return raw === null ? initialView() : parseViewRecord(raw, new Set(seed.sectionKeys));
  } catch {
    return initialView();
  }
}

export function createViewStore(seed: ViewStoreSeed): ViewStore {
  const mainKeys = mainSectionKeys(seed.sectionKeys);
  const known = new Set(mainKeys);
  let state: ViewState = readRemembered({ ...seed, sectionKeys: mainKeys });
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
        publish(collapseSections(state, mainKeys));
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
    toggleSettings: () => {
      publish(toggleSettings(state));
    },
    reveal: (key) => {
      publish(
        known.has(key) || isTemplateSettingsKey(key)
          ? revealSectionView(state, key)
          : withOnlyPinned(state, false),
      );
    },
  };
}
