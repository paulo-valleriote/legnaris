export interface LeaveGuard {
  intercepts(): boolean;
  requestLeave(proceed: () => void): void;
}

let active: LeaveGuard | undefined;

export function registerLeaveGuard(guard: LeaveGuard): () => void {
  active = guard;
  return () => {
    if (active === guard) {
      active = undefined;
    }
  };
}

export function leaveIsGuarded(): boolean {
  return active?.intercepts() ?? false;
}

export function requestLeave(proceed: () => void): void {
  if (active === undefined || !active.intercepts()) {
    proceed();
    return;
  }
  active.requestLeave(proceed);
}
