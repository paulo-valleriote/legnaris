/**
 * The one action → key map the shell dispatches through (WSC-20, WSC-36).
 *
 * The map is plain data — a later feature rebinds a key by editing it — and the
 * registry holds no domain state. A mounted sheet screen registers its handlers
 * through {@link useSheetShortcuts} and they are gone on unmount, so a stale
 * action can never outlive its screen. The screen binds no listener of its own.
 */

import { useEffect, useRef } from 'react';

/** The actions the shell dispatches (WSC-20, WSC-36). */
export type ShortcutAction = 'saveSheet' | 'refreshSheet';

/** One action's default key binding. */
export interface ShortcutSpec {
  /** The `KeyboardEvent.key` value, compared case-insensitively. */
  readonly key: string;
  /** Whether the platform control key (Ctrl/Cmd) must be held. */
  readonly ctrl: boolean;
}

/** The default action → key map (`web/AGENTS.md`). */
export const SHORTCUTS = {
  saveSheet: { key: 's', ctrl: true },
  refreshSheet: { key: 'F5', ctrl: false },
} as const satisfies Record<ShortcutAction, ShortcutSpec>;

/** The handlers a mounted sheet screen registers (WSC-20, WSC-36). */
export interface SheetShortcutHandlers {
  readonly onSave: () => void;
  readonly onRefresh?: () => void;
}

let registered: SheetShortcutHandlers | undefined;

/** Resolves a keydown to an action, or `null` when no binding matches (WSC-20, WSC-36). */
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

/** Invokes the registered handler for [action], or does nothing when none is registered. */
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

/**
 * Registers the mounted sheet screen's handlers and binds the one document
 * keydown listener (WSC-20, WSC-36).
 *
 * A matched shortcut's browser default is prevented while a screen is mounted
 * — so `Ctrl+S` never opens "Save page" and `F5` never reloads the app — and
 * the listener is removed with the screen, so the default is untouched when no
 * sheet is open.
 */
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
