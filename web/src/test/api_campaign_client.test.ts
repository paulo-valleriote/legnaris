import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const BASE = 'http://api.test';
const CAMPAIGN_ID = '0190c5a0-0000-7000-8000-000000000001';
const SHEET_ID = '0190c5a0-0000-7000-8000-000000000002';
const USER_ID = '0190c5a0-0000-7000-8000-000000000003';

const CAMPAIGN_VIEW = {
  id: CAMPAIGN_ID,
  name: 'Ember Coast',
  template_id: 'dnd5e',
  status: 'active',
  version: 4,
  gm_count: 1,
  player_count: 1,
  viewer_role: 'gm',
  viewer_owner: true,
  members: [
    {
      user_id: USER_ID,
      username: 'mara',
      role: 'gm',
      owner: true,
      joined_at: '2026-11-17T10:00:00Z',
      character_name: '',
    },
  ],
  created_at: '2026-11-17T10:00:00Z',
  updated_at: '2026-11-17T10:00:00Z',
};

const SHEET_DTO = {
  id: SHEET_ID,
  template_id: 'dnd5e',
  name: 'Brenna',
  data: { 'stats.hp': 12 },
  created_at: '2026-11-17T10:00:00Z',
  updated_at: '2026-11-17T10:00:00Z',
};

function clientWith(...outcomes: Parameters<typeof createFakeTransport>) {
  const fake = createFakeTransport(...outcomes);
  const client = createApiClient({
    baseUrl: BASE,
    transport: fake.transport,
    retryDelayMs: 0,
    sleep: () => Promise.resolve(),
  });
  return { client, fake };
}

describe('campaign API client (LGN-218)', () => {
  it('Happy path: listCampaigns GETs /campaigns and decodes the summaries', async () => {
    const { client, fake } = clientWith(
      successOutcome([
        {
          id: CAMPAIGN_ID,
          name: 'Ember Coast',
          template_id: 'dnd5e',
          version: 4,
          viewer_role: 'player',
          viewer_owner: false,
          updated_at: '2026-11-17T10:00:00Z',
        },
      ]),
    );

    const summaries = await client.listCampaigns();

    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${BASE}/campaigns`);
    expect(summaries).toEqual([
      expect.objectContaining({ id: CAMPAIGN_ID, viewerRole: 'player', viewerOwner: false }),
    ]);
  });

  it('Edge: listCampaigns rejects a payload that is not an array', async () => {
    const { client } = clientWith(successOutcome({ id: CAMPAIGN_ID }));

    await expect(client.listCampaigns()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Edge: a campaign with an unknown role is refused as drift, not shown', async () => {
    const { client } = clientWith(successOutcome({ ...CAMPAIGN_VIEW, viewer_role: 'spectator' }));

    await expect(client.getCampaign(CAMPAIGN_ID)).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Happy path: createCampaign POSTs the name and template id in snake case', async () => {
    const { client, fake } = clientWith(successOutcome(CAMPAIGN_VIEW, 201));

    const campaign = await client.createCampaign({ name: 'Ember Coast', templateId: 'dnd5e' });

    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE}/campaigns`);
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({
      name: 'Ember Coast',
      template_id: 'dnd5e',
    });
    expect(campaign.members[0]).toEqual(
      expect.objectContaining({ userId: USER_ID, owner: true, characterName: '' }),
    );
  });

  it('Error: createCampaign at the Plan limit maps PLAN_LIMIT_REACHED to its own kind', async () => {
    const { client } = clientWith(errorOutcome('PLAN_LIMIT_REACHED', 'limit', 403));

    const failure = await client
      .createCampaign({ name: 'x', templateId: 'dnd5e' })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({ kind: 'planLimitReached', code: 'PLAN_LIMIT_REACHED' });
  });

  it.each([
    ['promote', 'promote'],
    ['demote', 'demote'],
  ] as const)(
    'Happy path: %s posts to the member path and returns the campaign',
    async (action, suffix) => {
      const { client, fake } = clientWith(successOutcome(CAMPAIGN_VIEW));

      await client.changeMemberRole(CAMPAIGN_ID, USER_ID, action);

      expect(fake.lastRequest?.method).toBe('POST');
      expect(fake.lastRequest?.url).toBe(
        `${BASE}/campaigns/${CAMPAIGN_ID}/members/${USER_ID}/${suffix}`,
      );
    },
  );

  it('Happy path: removeMember posts to the remove path', async () => {
    const { client, fake } = clientWith(successOutcome(CAMPAIGN_VIEW));

    await client.removeMember(CAMPAIGN_ID, USER_ID);

    expect(fake.lastRequest?.url).toBe(
      `${BASE}/campaigns/${CAMPAIGN_ID}/members/${USER_ID}/remove`,
    );
  });

  it('Happy path: leave and delete send their verbs and accept a body-less success', async () => {
    const leave = clientWith({ status: 204, body: '' });
    await leave.client.leaveCampaign(CAMPAIGN_ID);
    expect(leave.fake.lastRequest?.method).toBe('POST');
    expect(leave.fake.lastRequest?.url).toBe(`${BASE}/campaigns/${CAMPAIGN_ID}/leave`);

    const remove = clientWith({
      status: 202,
      body: JSON.stringify({ data: { status: 'deleting' } }),
    });
    await remove.client.deleteCampaign(CAMPAIGN_ID);
    expect(remove.fake.lastRequest?.method).toBe('DELETE');
    expect(remove.fake.lastRequest?.url).toBe(`${BASE}/campaigns/${CAMPAIGN_ID}`);
  });

  it('Error: deleting a campaign as a player returns CAMPAIGN_FORBIDDEN', async () => {
    const { client } = clientWith(errorOutcome('CAMPAIGN_FORBIDDEN', 'no', 403));

    await expect(client.deleteCampaign(CAMPAIGN_ID)).rejects.toMatchObject({
      kind: 'campaignForbidden',
    });
  });

  it('Happy path: linkCharacter PUTs the sheet id and unlinkCharacter DELETEs the character', async () => {
    const link = clientWith(successOutcome(CAMPAIGN_VIEW));
    await link.client.linkCharacter(CAMPAIGN_ID, SHEET_ID);
    expect(link.fake.lastRequest?.method).toBe('PUT');
    expect(link.fake.lastRequest?.url).toBe(`${BASE}/campaigns/${CAMPAIGN_ID}/character`);
    expect(JSON.parse(link.fake.lastRequest?.body ?? '')).toEqual({ sheet_id: SHEET_ID });

    const unlink = clientWith(successOutcome(CAMPAIGN_VIEW));
    await unlink.client.unlinkCharacter(CAMPAIGN_ID);
    expect(unlink.fake.lastRequest?.method).toBe('DELETE');
    expect(unlink.fake.lastRequest?.url).toBe(`${BASE}/campaigns/${CAMPAIGN_ID}/character`);
  });

  it('Error: linking a sheet of another template returns TEMPLATE_MISMATCH', async () => {
    const { client } = clientWith(errorOutcome('TEMPLATE_MISMATCH', 'no', 422));

    await expect(client.linkCharacter(CAMPAIGN_ID, SHEET_ID)).rejects.toMatchObject({
      kind: 'templateMismatch',
    });
  });

  it('Error: linking a sheet already in a campaign returns SHEET_ALREADY_LINKED', async () => {
    const { client } = clientWith(errorOutcome('SHEET_ALREADY_LINKED', 'no', 409));

    await expect(client.linkCharacter(CAMPAIGN_ID, SHEET_ID)).rejects.toMatchObject({
      kind: 'sheetAlreadyLinked',
    });
  });

  it('Happy path: listCampaignSheets GETs the GM sheet list', async () => {
    const { client, fake } = clientWith(
      successOutcome([
        {
          id: SHEET_ID,
          name: 'Brenna',
          template_id: 'dnd5e',
          owner_id: USER_ID,
          updated_at: '2026-11-17T10:00:00Z',
        },
      ]),
    );

    const sheets = await client.listCampaignSheets(CAMPAIGN_ID);

    expect(fake.lastRequest?.url).toBe(`${BASE}/campaigns/${CAMPAIGN_ID}/sheets`);
    expect(sheets).toEqual([expect.objectContaining({ id: SHEET_ID, ownerId: USER_ID })]);
  });

  it('Happy path: getCampaignSheet sends If-None-Match and returns the sheet with its ETag', async () => {
    const { client, fake } = clientWith({
      status: 200,
      body: JSON.stringify({ data: SHEET_DTO }),
      etag: '"3.1.gm"',
    });

    const result = await client.getCampaignSheet(CAMPAIGN_ID, SHEET_ID, '"2.1.gm"');

    expect(fake.lastRequest?.headers).toEqual({ 'if-none-match': '"2.1.gm"' });
    expect(result).toEqual({
      notModified: false,
      sheet: expect.objectContaining({ id: SHEET_ID, name: 'Brenna' }),
      etag: '"3.1.gm"',
    });
  });

  it('Edge: getCampaignSheet sends no If-None-Match on the first read', async () => {
    const { client, fake } = clientWith(successOutcome(SHEET_DTO));

    await client.getCampaignSheet(CAMPAIGN_ID, SHEET_ID, undefined);

    expect(fake.lastRequest?.headers).toEqual({});
  });

  it('Happy path: a 304 keeps the cached view by reporting notModified with no body', async () => {
    const { client } = clientWith({ status: 304, body: '', etag: '"2.1.gm"' });

    await expect(client.getCampaignSheet(CAMPAIGN_ID, SHEET_ID, '"2.1.gm"')).resolves.toEqual({
      notModified: true,
    });
  });

  it('Error: a 404 on the GM sheet read maps to CAMPAIGN_NOT_FOUND with its own kind', async () => {
    const { client } = clientWith(errorOutcome('CAMPAIGN_NOT_FOUND', 'gone', 404));

    await expect(client.getCampaignSheet(CAMPAIGN_ID, SHEET_ID, '"2.1.gm"')).rejects.toMatchObject({
      kind: 'campaignNotFound',
    });
  });

  it('Edge: a 304 carries no body, so a 200 with an empty body is still refused as unreadable', async () => {
    const { client } = clientWith({ status: 200, body: '' });

    await expect(client.getCampaignSheet(CAMPAIGN_ID, SHEET_ID, undefined)).rejects.toBeInstanceOf(
      DefensiveParseFailure,
    );
  });

  it('Happy path: createCampaign POSTs the name and game system in snake case', async () => {
    const { client, fake } = clientWith(
      successOutcome({ ...CAMPAIGN_VIEW, game_system: { id: 'fifth', name: '5E' } }, 201),
    );

    await client.createCampaign({ name: 'Ember Coast', gameSystem: 'fifth' });

    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({
      name: 'Ember Coast',
      game_system: 'fifth',
    });
  });

  it('Happy path: a legacy campaign payload falls back to its template id for name and scope', async () => {
    const { client } = clientWith(successOutcome(CAMPAIGN_VIEW));

    const campaign = await client.getCampaign(CAMPAIGN_ID);

    expect(campaign.templateId).toBe('dnd5e');
    expect(campaign.scopeName).toBe('dnd5e');
    expect(campaign.templateIds).toEqual(['dnd5e']);
    expect(campaign.gameSystem).toBeUndefined();
  });

  it('Happy path: a Game system campaign payload decodes its scope without a template id', async () => {
    const { client } = clientWith(
      successOutcome({
        ...CAMPAIGN_VIEW,
        template_id: undefined,
        game_system: { id: 'fifth', name: '5E' },
        scope_name: '5E',
        template_ids: ['fifth_character_v1', 'fifth_statblock_v1'],
      }),
    );

    const campaign = await client.getCampaign(CAMPAIGN_ID);

    expect(campaign.templateId).toBeUndefined();
    expect(campaign.scopeName).toBe('5E');
    expect(campaign.templateIds).toEqual(['fifth_character_v1', 'fifth_statblock_v1']);
    expect(campaign.gameSystem).toEqual({ id: 'fifth', name: '5E' });
  });

  it('Edge: an empty scope template list decodes as empty', async () => {
    const { client } = clientWith(
      successOutcome({
        ...CAMPAIGN_VIEW,
        scope_name: '5E',
        template_ids: [],
        game_system: { id: 'fifth', name: '5E' },
      }),
    );

    expect((await client.getCampaign(CAMPAIGN_ID)).templateIds).toEqual([]);
  });

  it('Error: a campaign payload with neither a game system nor a template id is refused', async () => {
    const { client } = clientWith(
      successOutcome({
        ...CAMPAIGN_VIEW,
        template_id: undefined,
        scope_name: 'Nowhere',
        template_ids: [],
      }),
    );

    await expect(client.getCampaign(CAMPAIGN_ID)).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a malformed scope template list is refused', async () => {
    const { client } = clientWith(successOutcome({ ...CAMPAIGN_VIEW, template_ids: 'dnd5e' }));

    await expect(client.getCampaign(CAMPAIGN_ID)).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Happy path: a summary decodes the same scope fields as the detail', async () => {
    const { client } = clientWith(
      successOutcome([
        {
          ...CAMPAIGN_VIEW,
          template_id: undefined,
          game_system: { id: 'fifth', name: '5E' },
          scope_name: '5E',
          template_ids: ['fifth_character_v1'],
        },
      ]),
    );

    const [summary] = await client.listCampaigns();

    expect(summary).toMatchObject({
      scopeName: '5E',
      templateIds: ['fifth_character_v1'],
      gameSystem: { id: 'fifth', name: '5E' },
    });
    expect(summary?.templateId).toBeUndefined();
  });

  it('Happy path: a legacy summary falls back to its template id', async () => {
    const { client } = clientWith(successOutcome([CAMPAIGN_VIEW]));

    const [summary] = await client.listCampaigns();

    expect(summary).toMatchObject({
      scopeName: 'dnd5e',
      templateIds: ['dnd5e'],
      templateId: 'dnd5e',
    });
  });
});
