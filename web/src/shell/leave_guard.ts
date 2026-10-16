/**
 * The shell-level leave registry (WSC-37).
 *
 * A screen that owns unsaved work registers a guard while it is mounted and the
 * shell's navigation asks that guard before it leaves. The plain router the
 * shell mounts has no `useBlocker`, so the interception is a small module-level
 * registry the navigation calls directly rather than a router feature.
 */

/** The one guard an unsaved-work surface registers while it is mounted (WSC-37). */
export interface LeaveGuard {
  /** True when the guard intercepts a navigation: it confirms, or is inert while in flight (WSC-37). */
  intercepts(): boolean;
  /** Handles an intercepted leave: opens the confirmation, or does nothing while in flight (WSC-37). */
  requestLeave(proceed: () => void): void;
}

let active: LeaveGuard | undefined;

/** Registers [guard] as the active leave guard and returns its unregister (WSC-37). */
export function registerLeaveGuard(guard: LeaveGuard): () => void {
  active = guard;
  return () => {
    if (active === guard) {
      active = undefined;
    }
  };
}

/** True when the active guard intercepts a navigation before it runs (WSC-37). */
export function leaveIsGuarded(): boolean {
  return active?.intercepts() ?? false;
}

/**
 * Runs [proceed] at once when nothing guards the navigation, or delegates the
 * intercepted leave to the active guard otherwise (WSC-37).
 */
export function requestLeave(proceed: () => void): void {
  if (active === undefined || !active.intercepts()) {
    proceed();
    return;
  }
  active.requestLeave(proceed);
}
