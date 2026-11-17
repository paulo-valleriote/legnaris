import { describe, expect, it } from 'vitest';
import {
  atOwnedLimit,
  eligibleNpcSheetIds,
  eligibleSheetIds,
  isDeleting,
  isSelf,
  memberActions,
  unlinkedNpcSheetIds,
} from '../campaign/campaign_view';

const ME = 'me-user';
const OTHER = 'other-user';

type Role = 'gm' | 'player';

function viewer(viewerRole: Role, viewerOwner: boolean) {
  return { viewerUserId: ME, viewerRole, viewerOwner };
}

function member(role: Role, owner: boolean, userId = OTHER) {
  return { userId, role, owner };
}

describe('campaign view rules (LGN-218)', () => {
  it.each([
    ['owner viewer, GM member', viewer('gm', true), member('gm', false), ['demote', 'remove']],
    [
      'owner viewer, player member',
      viewer('gm', true),
      member('player', false),
      ['promote', 'remove'],
    ],
    ['owner viewer, owner row', viewer('gm', true), member('gm', true), []],
    ['GM viewer, player member', viewer('gm', false), member('player', false), ['remove']],
    ['GM viewer, GM member', viewer('gm', false), member('gm', false), []],
    ['GM viewer, owner row', viewer('gm', false), member('gm', true), []],
    ['player viewer, player member', viewer('player', false), member('player', false), []],
    ['player viewer, GM member', viewer('player', false), member('gm', false), []],
  ] as const)('Happy path: %s gets the right actions', (_name, v, m, expected) => {
    expect([...memberActions(v, m)]).toEqual([...expected]);
  });

  it('Edge: an owner flag on a player row still hides every action on that row', () => {
    expect(memberActions(viewer('gm', true), member('player', true))).toEqual([]);
  });

  it('Edge: the viewer own row has no actions, even for the owner or a GM', () => {
    expect(memberActions(viewer('gm', true), member('player', false, ME))).toEqual([]);
    expect(memberActions(viewer('gm', false), member('player', false, ME))).toEqual([]);
  });

  it('Edge: an empty viewer id never matches a member, so no row is taken for self', () => {
    expect(isSelf({ viewerUserId: '' }, { userId: '' })).toBe(false);
    expect(isSelf({ viewerUserId: ME }, { userId: OTHER })).toBe(false);
    expect(isSelf({ viewerUserId: ME }, { userId: ME })).toBe(true);
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

  it('Happy path: only sheets of the Campaign Template that are free or already here are eligible', () => {
    const campaign = { id: 'c1', templateId: 'fantasy_v1' };
    const sheets = [
      { id: 's-free', templateId: 'fantasy_v1' },
      { id: 's-here', templateId: 'fantasy_v1', campaignId: 'c1' },
      { id: 's-elsewhere', templateId: 'fantasy_v1', campaignId: 'c2' },
      { id: 's-other-template', templateId: 'other_v1' },
    ];
    expect(eligibleSheetIds(sheets, campaign)).toEqual(['s-free', 's-here']);
  });

  it('Edge: no sheets gives an empty choice', () => {
    expect(eligibleSheetIds([], { id: 'c1', templateId: 'fantasy_v1' })).toEqual([]);
  });

  it('Happy path: NPC and Monster sheets of the Campaign Template are eligible, even if linked elsewhere', () => {
    const sheets = [
      { id: 's-here', templateId: 'fantasy_v1' },
      { id: 's-elsewhere', templateId: 'fantasy_v1' },
      { id: 's-other-template', templateId: 'other_v1' },
    ];
    expect(eligibleNpcSheetIds(sheets, { templateId: 'fantasy_v1' })).toEqual([
      's-here',
      's-elsewhere',
    ]);
  });

  it('Edge: no owned NPC or Monster sheets gives an empty choice', () => {
    expect(eligibleNpcSheetIds([], { templateId: 'fantasy_v1' })).toEqual([]);
  });

  it('Happy path: sheets already linked to the Campaign are removed from the choice', () => {
    const sheets = [
      { id: 's-linked', templateId: 'fantasy_v1' },
      { id: 's-free', templateId: 'fantasy_v1' },
      { id: 's-other-template', templateId: 'other_v1' },
    ];
    expect(unlinkedNpcSheetIds(sheets, { templateId: 'fantasy_v1' }, ['s-linked'])).toEqual([
      's-free',
    ]);
  });

  it('Edge: no links and no sheets leave nothing to offer', () => {
    expect(unlinkedNpcSheetIds([], { templateId: 'fantasy_v1' }, [])).toEqual([]);
    expect(
      unlinkedNpcSheetIds(
        [{ id: 's-linked', templateId: 'fantasy_v1' }],
        { templateId: 'fantasy_v1' },
        ['s-linked'],
      ),
    ).toEqual([]);
  });
});
