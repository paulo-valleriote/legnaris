import { describe, expect, it } from 'vitest';
import { atOwnedLimit, isDeleting, memberActions } from '../campaign/campaign_view';

describe('campaign view rules (LGN-218)', () => {
  it.each([
    [
      'owner viewer, GM member',
      { viewerRole: 'gm', viewerOwner: true },
      { role: 'gm', owner: false },
      ['demote', 'remove'],
    ],
    [
      'owner viewer, player member',
      { viewerRole: 'gm', viewerOwner: true },
      { role: 'player', owner: false },
      ['promote', 'remove'],
    ],
    [
      'owner viewer, owner row',
      { viewerRole: 'gm', viewerOwner: true },
      { role: 'gm', owner: true },
      [],
    ],
    [
      'GM viewer, player member',
      { viewerRole: 'gm', viewerOwner: false },
      { role: 'player', owner: false },
      ['remove'],
    ],
    [
      'GM viewer, GM member',
      { viewerRole: 'gm', viewerOwner: false },
      { role: 'gm', owner: false },
      [],
    ],
    [
      'GM viewer, owner row',
      { viewerRole: 'gm', viewerOwner: false },
      { role: 'gm', owner: true },
      [],
    ],
    [
      'player viewer, player member',
      { viewerRole: 'player', viewerOwner: false },
      { role: 'player', owner: false },
      [],
    ],
    [
      'player viewer, GM member',
      { viewerRole: 'player', viewerOwner: false },
      { role: 'gm', owner: false },
      [],
    ],
  ] as const)('Happy path: %s gets the right actions', (_name, viewer, member, expected) => {
    expect([...memberActions(viewer, member)]).toEqual([...expected]);
  });

  it('Edge: an owner flag on a player row still hides every action on that row', () => {
    expect(
      memberActions({ viewerRole: 'gm', viewerOwner: true }, { role: 'player', owner: true }),
    ).toEqual([]);
  });

  it.each([
    ['active', false],
    ['deleting', true],
  ] as const)('Edge: status %s reads as deleting=%s', (status, expected) => {
    expect(isDeleting({ status })).toBe(expected);
  });

  it.each([
    [0, 1, false],
    [1, 1, true],
    [2, 1, true],
    [4, 5, false],
    [5, 5, true],
    [0, 0, true],
  ] as const)('Edge: used %i of limit %i at the limit=%s', (used, limit, expected) => {
    expect(atOwnedLimit({ used, limit })).toBe(expected);
  });
});
