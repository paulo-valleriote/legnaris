import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure } from '../api/errors';
import { isSheetLimitReached, parsePlanUsage } from '../api/models/plan';

const FREE_PAYLOAD = {
  plan: 'free',
  sheets: {
    character: { used: 1, limit: 1 },
    npc: { used: 0, limit: 2 },
    enemy: { used: 2, limit: 2 },
  },
};

describe('plan usage model (LGN-183)', () => {
  it('Happy path: decodes the plan and the usage of every kind', () => {
    const usage = parsePlanUsage(FREE_PAYLOAD);

    expect(usage.plan).toBe('free');
    expect(usage.sheets.character).toEqual({ used: 1, limit: 1 });
    expect(usage.sheets.npc).toEqual({ used: 0, limit: 2 });
  });

  it('Edge: a kind the payload omits has no entry rather than a made-up one', () => {
    const usage = parsePlanUsage({ plan: 'paid', sheets: { character: { used: 0, limit: 10 } } });

    expect(usage.sheets.character).toEqual({ used: 0, limit: 10 });
    expect(usage.sheets.npc).toBeUndefined();
  });

  it('Edge: a kind the client does not know is dropped, not rejected', () => {
    const usage = parsePlanUsage({
      plan: 'free',
      sheets: { character: { used: 0, limit: 1 }, vehicle: { used: 9, limit: 9 } },
    });

    expect(Object.keys(usage.sheets)).toEqual(['character']);
  });

  it.each([
    { name: 'a non-object payload', data: 'nope' },
    { name: 'a missing sheets object', data: { plan: 'free' } },
    {
      name: 'a non-numeric used count',
      data: { plan: 'free', sheets: { character: { used: '1', limit: 1 } } },
    },
    { name: 'a missing plan', data: { sheets: {} } },
  ])('Error: $name is a defensive parse failure', ({ data }) => {
    expect(() => parsePlanUsage(data)).toThrow(DefensiveParseFailure);
  });

  it('Edge: the limit is reached exactly at the allowance, and never before it', () => {
    const usage = parsePlanUsage(FREE_PAYLOAD);

    expect(isSheetLimitReached(usage, 'character')).toBe(true);
    expect(isSheetLimitReached(usage, 'npc')).toBe(false);
    expect(isSheetLimitReached(usage, 'enemy')).toBe(true);
  });

  it('Edge: an unread or partial usage never reports the limit as reached', () => {
    expect(isSheetLimitReached(undefined, 'character')).toBe(false);
    expect(isSheetLimitReached(parsePlanUsage({ plan: 'free', sheets: {} }), 'character')).toBe(
      false,
    );
  });
});
