export interface SlotPool {
  readonly total: number;
  readonly expended: number;
  readonly remaining: number;
}

export interface SlotPip {
  readonly index: number;
  readonly spent: boolean;
}

function slotCount(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
}

export function slotPool(total: unknown, expended: unknown): SlotPool {
  const capacity = slotCount(total);
  const spent = Math.min(capacity, slotCount(expended));
  return { total: capacity, expended: spent, remaining: capacity - spent };
}

export function spendSlot(pool: SlotPool): SlotPool {
  if (pool.remaining === 0) {
    return pool;
  }
  return { total: pool.total, expended: pool.expended + 1, remaining: pool.remaining - 1 };
}

export function recoverSlot(pool: SlotPool): SlotPool {
  if (pool.expended === 0) {
    return pool;
  }
  const expended = pool.expended - 1;
  return { total: pool.total, expended, remaining: pool.total - expended };
}

export function slotPips(pool: SlotPool): readonly SlotPip[] {
  return Array.from({ length: pool.total }, (_, index) => ({
    index,
    spent: index >= pool.remaining,
  }));
}
