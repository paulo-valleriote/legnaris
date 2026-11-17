import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { parseNpcLink } from '../api/models/campaign';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const BASE_URL = 'http://api.test';
const CAMPAIGN_ID = '0190c5a0-0000-7000-8000-00000000c001';
const SHEET_ID = '0190c5a0-0000-7000-8000-00000000c002';
const LINK_ID = '0190c5a0-0000-7000-8000-00000000c003';

const LINK = {
  id: LINK_ID,
  sheet_id: SHEET_ID,
  name: 'Goblin',
  kind: 'npc',
  owner_id: '0190c5a0-0000-7000-8000-00000000c004',
  owner_name: 'mara',
  revealed: false,
  linked_at: '2026-11-17T10:00:00Z',
  is_owner: true,
};

const SHEET = {
  id: SHEET_ID,
  template_id: 'fantasy_v1',
  kind: 'npc',
  name: 'Goblin',
  data: { stats: { strength: 8 } },
  created_at: '2026-11-17T10:00:00Z',
  updated_at: '2026-11-17T10:00:00Z',
};

function clientFor(...outcomes: Parameters<typeof createFakeTransport>) {
  const fake = createFakeTransport(...outcomes);
  const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
  return { client, fake };
}

describe('api client: NPC and Monster links (LGN-219)', () => {
  it('Happy path: linkNpc puts the sheet id and decodes the link', async () => {
    const { client, fake } = clientFor(successOutcome(LINK));

    const link = await client.linkNpc(CAMPAIGN_ID, SHEET_ID);

    expect(link).toEqual({
      id: LINK_ID,
      sheetId: SHEET_ID,
      name: 'Goblin',
      kind: 'npc',
      ownerId: LINK.owner_id,
      ownerName: 'mara',
      revealed: false,
      linkedAt: LINK.linked_at,
      isOwner: true,
    });
    expect(fake.lastRequest?.method).toBe('PUT');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/campaigns/${CAMPAIGN_ID}/npcs`);
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ sheet_id: SHEET_ID });
  });

  it('Happy path: unlinkNpc deletes the link path and resolves on 204', async () => {
    const { client, fake } = clientFor({ status: 204, body: '' });

    await expect(client.unlinkNpc(CAMPAIGN_ID, SHEET_ID)).resolves.toBeUndefined();

    expect(fake.lastRequest?.method).toBe('DELETE');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/campaigns/${CAMPAIGN_ID}/npcs/${SHEET_ID}`);
    expect(fake.lastRequest?.body).toBeUndefined();
  });

  it('Happy path: listNpcLinks reads the collection', async () => {
    const { client, fake } = clientFor(
      successOutcome([LINK, { ...LINK, id: LINK_ID, sheet_id: SHEET_ID, kind: 'monster' }]),
    );

    const links = await client.listNpcLinks(CAMPAIGN_ID);

    expect(links).toHaveLength(2);
    expect(links[1]?.kind).toBe('monster');
    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/campaigns/${CAMPAIGN_ID}/npcs`);
  });

  it('Happy path: getCampaignNpc sends If-None-Match and reports a 304 as not modified', async () => {
    const { client, fake } = clientFor({ status: 304, body: '', etag: '"3.1.gm"' });

    const result = await client.getCampaignNpc(CAMPAIGN_ID, SHEET_ID, '"3.1.gm"');

    expect(result).toEqual({ notModified: true });
    expect(fake.lastRequest?.headers).toEqual({ 'if-none-match': '"3.1.gm"' });
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/campaigns/${CAMPAIGN_ID}/npcs/${SHEET_ID}`);
  });

  it('Happy path: getCampaignNpc decodes the sheet and keeps its ETag', async () => {
    const { client, fake } = clientFor({
      status: 200,
      body: JSON.stringify({ data: SHEET }),
      etag: '"4.1.gm"',
    });

    const result = await client.getCampaignNpc(CAMPAIGN_ID, SHEET_ID, undefined);

    expect(result).toMatchObject({ notModified: false, etag: '"4.1.gm"' });
    expect(result.notModified ? undefined : result.sheet.name).toBe('Goblin');
    expect(fake.lastRequest?.headers).toEqual({});
  });

  it('Happy path: duplicateSheet posts and decodes the copy', async () => {
    const copy = { ...SHEET, id: '0190c5a0-0000-7000-8000-00000000c00f', name: 'Goblin (copy)' };
    const { client, fake } = clientFor(successOutcome(copy, 201));

    const duplicated = await client.duplicateSheet(SHEET_ID);

    expect(duplicated.name).toBe('Goblin (copy)');
    expect(duplicated.kind).toBe('npc');
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/sheets/${SHEET_ID}/duplicate`);
  });

  it('Error: SHEET_KIND_MISMATCH is classified as sheetKindMismatch', async () => {
    const { client } = clientFor(errorOutcome('SHEET_KIND_MISMATCH', 'not linkable', 422));

    const failure = await client.linkNpc(CAMPAIGN_ID, SHEET_ID).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({
      kind: 'sheetKindMismatch',
      code: 'SHEET_KIND_MISMATCH',
      status: 422,
    });
  });

  it('Error: PLAN_LIMIT_REACHED is classified as planLimitReached for a refused duplicate', async () => {
    const { client } = clientFor(errorOutcome('PLAN_LIMIT_REACHED', 'at the limit', 403));

    const failure = await client.duplicateSheet(SHEET_ID).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({ kind: 'planLimitReached', status: 403 });
  });
});

describe('NPC link model', () => {
  it('Error: a character link or an unknown kind is drift rather than data', () => {
    expect(() => parseNpcLink({ ...LINK, kind: 'character' })).toThrow(DefensiveParseFailure);
    expect(() => parseNpcLink({ ...LINK, kind: 'dragon' })).toThrow(DefensiveParseFailure);
  });

  it('Error: a missing identifier, name, owner or flag is refused', () => {
    for (const key of ['id', 'sheet_id', 'name', 'owner_id', 'revealed', 'linked_at', 'is_owner']) {
      expect(() => parseNpcLink({ ...LINK, [key]: undefined })).toThrow(DefensiveParseFailure);
    }
  });

  it('Edge: an empty owner name decodes as empty text', () => {
    expect(parseNpcLink({ ...LINK, owner_name: undefined }).ownerName).toBe('');
  });
});
