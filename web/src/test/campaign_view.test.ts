import { describe, expect, it } from 'vitest';
import type { TemplateSummary } from '../api/models/template';
import {
  atOwnedLimit,
  campaignScopeOptions,
  createCampaignInput,
  eligibleNpcSheetIds,
  eligibleSheetIds,
  isDeleting,
  isSelf,
  memberActions,
  memberUsername,
  unlinkedNpcSheetIds,
} from '../campaign/campaign_view';

const GAME_SYSTEM = {
  id: 'fifth',
  name: '5E',
  compatibilityLabel: '5E compatible',
  notice: { label: 'SRD 5.2.1', path: '/srd' },
};

function template(
  id: string,
  name: string,
  gameSystem: TemplateSummary['gameSystem'] = undefined,
): TemplateSummary {
  return {
    id,
    name,
    kinds: ['character'],
    gameSystem,
    default: false,
    fields: {},
  };
}

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
    const campaign = { id: 'c1', templateIds: ['fantasy_v1'] };
    const sheets = [
      { id: 's-free', templateId: 'fantasy_v1' },
      { id: 's-here', templateId: 'fantasy_v1', campaignId: 'c1' },
      { id: 's-elsewhere', templateId: 'fantasy_v1', campaignId: 'c2' },
      { id: 's-other-template', templateId: 'other_v1' },
    ];
    expect(eligibleSheetIds(sheets, campaign)).toEqual(['s-free', 's-here']);
  });

  it('Happy path: a Sheet of any Template in the Campaign scope is eligible', () => {
    const campaign = { id: 'c1', templateIds: ['fifth_character_v1', 'fifth_statblock_v1'] };
    const sheets = [
      { id: 's-character', templateId: 'fifth_character_v1' },
      { id: 's-statblock', templateId: 'fifth_statblock_v1' },
      { id: 's-other', templateId: 'fantasy_v1' },
    ];
    expect(eligibleSheetIds(sheets, campaign)).toEqual(['s-character', 's-statblock']);
  });

  it('Edge: an empty scope template list makes no sheet eligible', () => {
    expect(
      eligibleSheetIds([{ id: 's-free', templateId: 'fantasy_v1' }], {
        id: 'c1',
        templateIds: [],
      }),
    ).toEqual([]);
  });

  it('Edge: no sheets gives an empty choice', () => {
    expect(eligibleSheetIds([], { id: 'c1', templateIds: ['fantasy_v1'] })).toEqual([]);
  });

  it('Happy path: NPC and Enemy sheets of the Campaign scope are eligible, even if linked elsewhere', () => {
    const sheets = [
      { id: 's-here', templateId: 'fantasy_v1' },
      { id: 's-elsewhere', templateId: 'fantasy_v1' },
      { id: 's-other-template', templateId: 'other_v1' },
    ];
    expect(eligibleNpcSheetIds(sheets, { templateIds: ['fantasy_v1'] })).toEqual([
      's-here',
      's-elsewhere',
    ]);
  });

  it('Edge: no owned NPC or Enemy sheets gives an empty choice', () => {
    expect(eligibleNpcSheetIds([], { templateIds: ['fantasy_v1'] })).toEqual([]);
  });

  it('Happy path: sheets already linked to the Campaign are removed from the choice', () => {
    const sheets = [
      { id: 's-linked', templateId: 'fantasy_v1' },
      { id: 's-free', templateId: 'fantasy_v1' },
      { id: 's-other-template', templateId: 'other_v1' },
    ];
    expect(unlinkedNpcSheetIds(sheets, { templateIds: ['fantasy_v1'] }, ['s-linked'])).toEqual([
      's-free',
    ]);
  });

  it('Edge: no links and no sheets leave nothing to offer', () => {
    expect(unlinkedNpcSheetIds([], { templateIds: ['fantasy_v1'] }, [])).toEqual([]);
    expect(
      unlinkedNpcSheetIds(
        [{ id: 's-linked', templateId: 'fantasy_v1' }],
        { templateIds: ['fantasy_v1'] },
        ['s-linked'],
      ),
    ).toEqual([]);
  });
});

describe('campaign scope options (LGN-401)', () => {
  it('Edge: no Templates offer no option', () => {
    expect(campaignScopeOptions([])).toEqual([]);
  });

  it('Happy path: a standalone Template offers itself by name', () => {
    expect(campaignScopeOptions([template('fantasy_v1', 'Fantasy Template')])).toEqual([
      { key: 'template:fantasy_v1', label: 'Fantasy Template', templateId: 'fantasy_v1' },
    ]);
  });

  it('Happy path: two Templates of one Game system give one option named after the system', () => {
    const options = campaignScopeOptions([
      template('fifth_character_v1', '5E Character', GAME_SYSTEM),
      template('fifth_statblock_v1', '5E Stat Block', GAME_SYSTEM),
    ]);

    expect(options).toEqual([{ key: 'game-system:fifth', label: '5E', gameSystemId: 'fifth' }]);
  });

  it('Edge: a Game system appearing after a standalone Template keeps the first-seen order', () => {
    const options = campaignScopeOptions([
      template('fantasy_v1', 'Fantasy Template'),
      template('fifth_character_v1', '5E Character', GAME_SYSTEM),
      template('fifth_statblock_v1', '5E Stat Block', GAME_SYSTEM),
      template('other_v1', 'Other Template'),
    ]);

    expect(options.map((option) => option.key)).toEqual([
      'template:fantasy_v1',
      'game-system:fifth',
      'template:other_v1',
    ]);
  });

  it('Edge: a repeated standalone Template offers one option', () => {
    expect(
      campaignScopeOptions([
        template('fantasy_v1', 'Fantasy Template'),
        template('fantasy_v1', 'Fantasy Template'),
      ]),
    ).toHaveLength(1);
  });

  it('Happy path: a Game system option sends only game_system', () => {
    expect(
      createCampaignInput('Ember Coast', {
        key: 'game-system:fifth',
        label: '5E',
        gameSystemId: 'fifth',
      }),
    ).toEqual({ name: 'Ember Coast', gameSystem: 'fifth' });
  });

  it('Happy path: a standalone Template option sends only template_id', () => {
    expect(
      createCampaignInput('Ember Coast', {
        key: 'template:fantasy_v1',
        label: 'Fantasy Template',
        templateId: 'fantasy_v1',
      }),
    ).toEqual({ name: 'Ember Coast', templateId: 'fantasy_v1' });
  });
});

describe('memberUsername', () => {
  const campaign = {
    members: [
      { userId: 'u-1', username: 'mara' },
      { userId: 'u-2', username: '' },
    ],
  } as unknown as Parameters<typeof memberUsername>[0];

  it.each([
    ['Happy path: a member resolves to their username', 'u-1', 'mara'],
    ['Edge: a member with an empty username resolves to nothing', 'u-2', undefined],
    ['Edge: an unknown user resolves to nothing', 'u-3', undefined],
    ['Edge: an empty id resolves to nothing', '', undefined],
  ])('%s', (_name, userId, expected) => {
    expect(memberUsername(campaign, userId)).toBe(expected);
  });

  it('Edge: a Campaign with no members resolves to nothing', () => {
    expect(memberUsername({ members: [] }, 'u-1')).toBeUndefined();
  });
});
