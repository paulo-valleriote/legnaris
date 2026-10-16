import { beforeAll, describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import type { ApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import { valueAtPath } from '../api/models/sheet';
import type { Sheet } from '../api/models/sheet';
import { createFetchTransport } from '../api/transport';
import type { Transport } from '../api/transport';

// The live suite is switchable at run time (unlike the build-time base address):
// with no API_BASE_URL it steps aside and the default run stays green (API-37);
// with one it must exercise the real service and never skip silently (API-39).
const configuredBaseUrl = process.env['API_BASE_URL'];

function liveRunDecision(configured: string | undefined): 'run' | 'skip' {
  return configured === undefined || configured.trim() === '' ? 'skip' : 'run';
}

const live = liveRunDecision(configuredBaseUrl) === 'run' ? describe : describe.skip;

function requireBaseUrl(): string {
  if (configuredBaseUrl === undefined || configuredBaseUrl.trim() === '') {
    throw new Error('API_BASE_URL is required to run the live contract suite');
  }

  return configuredBaseUrl;
}

function transportWithCredential(token: string): Transport {
  const base = createFetchTransport((input, init) => fetch(input, init));

  return (request) =>
    base({ ...request, headers: { ...request.headers, authorization: `Bearer ${token}` } });
}

async function authenticate(baseUrl: string): Promise<string> {
  const suffix = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  const email = `live-${suffix}@example.test`;
  const password = 'live-suite-password-1';

  const registered = await fetch(`${baseUrl}/auth/register`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ email, username: `live${suffix}`, password }),
  });
  expect(registered.status).toBe(201);

  const loggedIn = await fetch(`${baseUrl}/auth/login`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  expect(loggedIn.status).toBe(200);

  const pair = (await loggedIn.json()) as { data: { access_token: string } };
  return pair.data.access_token;
}

describe('live suite gating', () => {
  it('Edge: with no base address the live flows step aside (API-37)', () => {
    expect(liveRunDecision(undefined)).toBe('skip');
    expect(liveRunDecision('')).toBe('skip');
    expect(liveRunDecision('   ')).toBe('skip');
  });

  it('Edge: with a base address the live flows never step aside silently (API-37, API-39)', () => {
    expect(liveRunDecision('http://localhost:8080')).toBe('run');
    expect(liveRunDecision('https://api.example.test')).toBe('run');
  });
});

live('live backend contract', () => {
  let client: ApiClient;
  let created: Sheet;

  beforeAll(async () => {
    const baseUrl = requireBaseUrl();
    const token = await authenticate(baseUrl);
    client = createApiClient({ baseUrl, transport: transportWithCredential(token) });
    created = await client.createSheet({
      templateId: 'fantasy_v1',
      sheetName: `Live ${Date.now()}`,
    });
  });

  it('Happy path: creating a sheet returns a created sheet with derived values (API-38)', () => {
    expect(created.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    expect(created.templateId).toBe('fantasy_v1');
    expect(valueAtPath(created.data, 'modifiers.dexterityMod')).toBe(0);
    expect(typeof valueAtPath(created.data, 'derived.armorClass')).toBe('number');
  });

  it('Happy path: updating an editable field returns only recalculated effects (API-38)', async () => {
    const effects = await client.patchSheet(created.id, { stats: { dexterity: 14 } });

    expect(valueAtPath(effects, 'modifiers.dexterityMod')).toBe(2);
    expect(effects).not.toHaveProperty('stats');
  });

  it('Happy path: updating a derived field is refused as an invalid edit (API-38)', async () => {
    const failure = await client
      .patchSheet(created.id, { modifiers: { dexterityMod: 5 } })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe('invalidEdit');
    expect((failure as DomainFailure).code).toBe('INVALID_REQUEST');
  });

  it('Happy path: updating an unknown field is refused as a render mismatch (API-38)', async () => {
    const failure = await client
      .patchSheet(created.id, { stats: { nonexistentAbility: 1 } })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe('renderMismatch');
    expect((failure as DomainFailure).code).toBe('UNKNOWN_FIELD');
  });

  it('Happy path: retrieving a sheet that does not exist reports it missing (API-38)', async () => {
    const lastDigit = created.id.slice(-1);
    const absentId = `${created.id.slice(0, -1)}${lastDigit === 'a' ? 'b' : 'a'}`;

    const failure = await client.getSheet(absentId).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe('notFound');
    expect((failure as DomainFailure).status).toBe(404);
  });
});
