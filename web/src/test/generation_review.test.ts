import { describe, expect, it } from 'vitest';
import {
  PREVIEW_THROTTLE_MS,
  isLastGeneration,
  mergeCascade,
  previewDelay,
} from '../generate/generation_flow';

describe('previewDelay (LGN-196 throttled derived values)', () => {
  const tests = [
    {
      name: 'Happy path: the first preview is sent at once',
      last: undefined,
      now: 5_000,
      delay: 0,
    },
    {
      name: 'Edge: a preview a full window after the last one is sent at once',
      last: 1_000,
      now: 2_000,
      delay: 0,
    },
    {
      name: 'Edge: a preview inside the window waits for the rest of it',
      last: 1_000,
      now: 1_400,
      delay: 600,
    },
    {
      name: 'Edge: one millisecond inside the window still waits',
      last: 1_000,
      now: 1_999,
      delay: 1,
    },
    {
      name: 'Edge: a clock that went backwards waits the full window, never more',
      last: 5_000,
      now: 4_000,
      delay: PREVIEW_THROTTLE_MS,
    },
  ] as const;

  for (const tc of tests) {
    it(tc.name, () => {
      expect(previewDelay(tc.last, tc.now)).toBe(tc.delay);
    });
  }
});

describe('mergeCascade (LGN-196 recalculated values)', () => {
  it('Happy path: overwrites only the paths the cascade names', () => {
    const merged = mergeCascade(
      { stats: { strength: 12, modifier: 1 } },
      { stats: { modifier: 2 } },
    );
    expect(merged).toEqual({ stats: { strength: 12, modifier: 2 } });
  });

  it('Edge: an empty cascade returns the values unchanged', () => {
    const values = { stats: { strength: 12 } };
    expect(mergeCascade(values, {})).toEqual(values);
  });

  it('Edge: the input values are not mutated', () => {
    const values = { stats: { strength: 12 } };
    mergeCascade(values, { stats: { strength: 13 } });
    expect(values).toEqual({ stats: { strength: 12 } });
  });

  it('Edge: a prototype-polluting key in the cascade is ignored', () => {
    const cascade = JSON.parse(
      '{"__proto__": {"polluted": true}, "stats": {"strength": 9}}',
    ) as Record<string, unknown>;
    const merged = mergeCascade({}, cascade);
    expect(({} as Record<string, unknown>)['polluted']).toBeUndefined();
    expect(merged).toEqual({ stats: { strength: 9 } });
  });

  it('Edge: a non-object cascade leaves the values as they were', () => {
    expect(mergeCascade({ a: 1 }, null as unknown as Record<string, unknown>)).toEqual({ a: 1 });
  });
});

describe('isLastGeneration (LGN-196 last-one confirmation)', () => {
  it('Happy path: the server flag means the next regenerate uses the last one', () => {
    expect(isLastGeneration({ lastFreeGeneration: true })).toBe(true);
  });

  it('Edge: a plan without the flag is not the last one', () => {
    expect(isLastGeneration({ lastFreeGeneration: false })).toBe(false);
  });

  it('Edge: a plan not yet read never asks for confirmation', () => {
    expect(isLastGeneration(undefined)).toBe(false);
  });
});
