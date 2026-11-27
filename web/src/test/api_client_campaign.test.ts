import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import { parseCampaign, parseJoinedCampaign } from '../api/models/campaign';
import { campaignView } from './campaign_helpers';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const CODE = 'Abcdefghijklmnopqrstuvwx';
const CAMPAIGN_WIRE = campaignView();
const CAMPAIGN = parseCampaign(CAMPAIGN_WIRE);

function clientFor(...outcomes: Parameters<typeof createFakeTransport>) {
  const fake = createFakeTransport(...outcomes);
  const client = createApiClient({ baseUrl: 'http://api.test', transport: fake.transport });
  return { client, fake };
}

describe('api client: joinCampaign (LGN-200)', () => {
  it('Happy path: posts the code to /campaigns/join and returns the joined campaign', async () => {
    const { client, fake } = clientFor(successOutcome(CAMPAIGN_WIRE));

    await expect(client.joinCampaign({ code: CODE })).resolves.toEqual(CAMPAIGN);

    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe('http://api.test/campaigns/join');
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ code: CODE });
  });

  it('Happy path: sends the password alongside the code when one is given', async () => {
    const { client, fake } = clientFor(successOutcome(CAMPAIGN_WIRE));

    await client.joinCampaign({ code: CODE, password: 'correct horse' });

    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({
      code: CODE,
      password: 'correct horse',
    });
  });

  it('Edge: an undefined password is omitted from the body rather than sent as null', async () => {
    const { client, fake } = clientFor(successOutcome(CAMPAIGN_WIRE));

    await client.joinCampaign({ code: CODE, password: undefined });

    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ code: CODE });
  });

  it('Edge: an empty password is sent as given, so the server can refuse it', async () => {
    const { client, fake } = clientFor(successOutcome(CAMPAIGN_WIRE));

    await client.joinCampaign({ code: CODE, password: '' });

    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ code: CODE, password: '' });
  });

  it('Edge: an answer without a campaign id is a defensive parse failure', () => {
    expect(() => parseCampaign({ name: 'No id' })).toThrow();
    expect(() => parseJoinedCampaign({ ...CAMPAIGN_WIRE, id: '' })).toThrow();
    expect(() => parseCampaign({ ...CAMPAIGN_WIRE, members: 'not-a-list' })).toThrow();
    expect(() => parseCampaign(null)).toThrow();
  });

  it.each([
    ['INVALID_INVITE', 404, 'invalidInvite'],
    ['PASSWORD_REQUIRED', 403, 'passwordRequired'],
    ['RATE_LIMITED', 429, 'rateLimited'],
    ['CAMPAIGN_FULL', 409, 'campaignFull'],
  ])('Error: %s is classified as a domain failure of kind %s', async (code, status, kind) => {
    const { client } = clientFor(errorOutcome(code, 'refused', status));

    const failure = await client.joinCampaign({ code: CODE }).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({ kind, code, status });
  });
});
