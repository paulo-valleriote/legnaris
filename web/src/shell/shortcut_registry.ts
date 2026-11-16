import { useEffect, useRef } from 'react';

export type ShortcutAction = 'saveSheet' | 'refreshSheet';

export interface ShortcutSpec {
  readonly key: string;
  readonly ctrl: boolean;
}

export const SHORTCUTS = {
  saveSheet: { key: 's', ctrl: true },
  refreshSheet: { key: 'F5', ctrl: false },
} as const satisfies Record<ShortcutAction, ShortcutSpec>;

export interface SheetShortcutHandlers {
  readonly onSave: () => void;
  readonly onRefresh?: () => void;
}

let registered: SheetShortcutHandlers | undefined;

export function matchShortcut(event: KeyboardEvent): ShortcutAction | null {
  const control = event.ctrlKey || event.metaKey;
  const key = event.key.toLowerCase();

  if (control && !event.altKey && key === SHORTCUTS.saveSheet.key) {
    return 'saveSheet';
  }
  if (!control && !event.altKey && event.key === SHORTCUTS.refreshSheet.key) {
    return 'refreshSheet';
  }
  return null;
}

export function dispatchShortcut(action: ShortcutAction): void {
  if (registered === undefined) {
    return;
  }
  if (action === 'saveSheet') {
    registered.onSave();
  } else {
    registered.onRefresh?.();
  }
}

export function useSheetShortcuts(handlers: SheetShortcutHandlers): void {
  const latest = useRef(handlers);

  useEffect(() => {
    latest.current = handlers;
  });

  useEffect(() => {
    registered = {
      onSave: () => latest.current.onSave(),
      onRefresh: () => latest.current.onRefresh?.(),
    };
    return () => {
      registered = undefined;
    };
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const action = matchShortcut(event);
      if (action === null) {
        return;
      }
      event.preventDefault();
      dispatchShortcut(action);
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);
}
