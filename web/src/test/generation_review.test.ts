import { describe, expect, it } from 'vitest';
import type { FieldNode } from '../api/models/template';
import {
  PREVIEW_THROTTLE_MS,
  isLastGeneration,
  mergeCascade,
  previewDelay,
  suggestedSheetName,
} from '../generate/generation_flow';

const leaf = (label: string): FieldNode => ({
  kind: 'leaf',
  schema: { type: 'text', label, isEditable: true, defaultValue: 0 },
});

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

describe('suggestedSheetName (LGN-196 Sheet name prefill)', () => {
  const fields = {
    identity: { kind: 'group', children: { characterName: leaf('Character name') } },
  } as unknown as Record<string, FieldNode>;

  it('Happy path: prefills from the character name leaf when the template has it', () => {
    expect(suggestedSheetName(fields, { identity: { characterName: 'Aragorn' } })).toBe('Aragorn');
  });

  it('Edge: surrounding whitespace is trimmed', () => {
    expect(suggestedSheetName(fields, { identity: { characterName: '  Aragorn  ' } })).toBe(
      'Aragorn',
    );
  });

  it('Edge: an empty generated name prefills nothing', () => {
    expect(suggestedSheetName(fields, { identity: { characterName: '   ' } })).toBe('');
  });

  it('Edge: a template without the leaf prefills nothing', () => {
    expect(suggestedSheetName({}, { identity: { characterName: 'Aragorn' } })).toBe('');
  });

  it('Edge: a non-text value is never used as a name', () => {
    expect(suggestedSheetName(fields, { identity: { characterName: 7 } })).toBe('');
  });

  it('Edge: a missing identity group in the values prefills nothing', () => {
    expect(suggestedSheetName(fields, {})).toBe('');
  });
});

describe('isLastGeneration (LGN-196 last-one confirmation)', () => {
  it('Happy path: one generation left means the next regenerate uses the last one', () => {
    expect(isLastGeneration({ kind: 'trial', remaining: 1 })).toBe(true);
  });

  it('Edge: several left is not the last one', () => {
    expect(isLastGeneration({ kind: 'trial', remaining: 2 })).toBe(false);
  });

  it('Edge: none left is not a last one to confirm, it is refused', () => {
    expect(isLastGeneration({ kind: 'trial', remaining: 0 })).toBe(false);
  });

  it('Edge: an unknown allowance never asks for confirmation', () => {
    expect(isLastGeneration({ kind: 'unknown' })).toBe(false);
  });
});
