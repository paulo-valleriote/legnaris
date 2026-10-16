import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';

const templatePayload = {
  id: 'fantasy_v1',
  name: 'Fantasy RPG',
  default: true,
  fields: {
    stats: {
      children: {
        strength: { schema: { type: 'number', label: 'Strength', is_editable: true, default: 10 } },
      },
    },
  },
};

const sheetPayload = {
  id: '01a0f967-f42b-710e-851f-8f293a634921',
  template_id: 'fantasy_v1',
  name: 'Aragorn',
  data: { stats: { dexterity: 16 } },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

describe('api client operations', () => {
  it('Happy path: health reads GET /health over the injected transport (API-01, API-07)', async () => {
    const fake = createFakeTransport(successOutcome({ status: 'ok' }));
    const payload = await clientWith(fake).health();

    expect(payload).toEqual({ status: 'ok' });
    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/health`);
    expect(fake.lastRequest?.headers).toEqual({});
    expect(fake.lastRequest?.body).toBeUndefined();
    expect(fake.lastRequest?.credentials).toBe('omit');
    expect(fake.attempts).toBe(1);
  });

  it('Happy path: listTemplates reads GET /templates and decodes a catalog (API-01, API-02, API-08)', async () => {
    const fake = createFakeTransport(successOutcome([templatePayload]));
    const payload = await clientWith(fake).listTemplates();

    expect(payload.map((entry) => entry.id)).toEqual(['fantasy_v1']);
    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/templates`);
  });

  it('Happy path: getTemplate reads GET /templates/:id and decodes the detail (API-01, API-08)', async () => {
    const fake = createFakeTransport(successOutcome(templatePayload));
    const payload = await clientWith(fake).getTemplate('fantasy_v1');

    expect(payload.name).toBe('Fantasy RPG');
    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/templates/fantasy_v1`);
  });

  it('Happy path: createSheet posts exactly the template identifier and the sheet name (API-05)', async () => {
    const fake = createFakeTransport(successOutcome(sheetPayload, 201));
    const payload = await clientWith(fake).createSheet({
      templateId: 'fantasy_v1',
      sheetName: 'Aragorn',
    });

    expect(payload.name).toBe('Aragorn');
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets`);
    expect(fake.lastRequest?.headers).toEqual({ 'content-type': 'application/json' });
    expect(fake.lastRequest?.body).toBe(
      JSON.stringify({ template_id: 'fantasy_v1', sheet_name: 'Aragorn' }),
    );
    expect(Object.keys(JSON.parse(fake.lastRequest?.body ?? '{}'))).toEqual([
      'template_id',
      'sheet_name',
    ]);
  });

  it('Happy path: getSheet reads GET /sheets/:id and decodes the sheet (API-01, API-10)', async () => {
    const fake = createFakeTransport(successOutcome(sheetPayload));
    const payload = await clientWith(fake).getSheet('01a0f967-f42b-710e-851f-8f293a634921');

    expect(payload.templateId).toBe('fantasy_v1');
    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/01a0f967-f42b-710e-851f-8f293a634921`);
    expect(fake.lastRequest?.headers).toEqual({});
  });

  it('Happy path: patchSheet sends exactly the change set and decodes the recalculated effects (API-06)', async () => {
    const fake = createFakeTransport(successOutcome({ modifiers: { dexterityMod: 3 } }));
    const payload = await clientWith(fake).patchSheet('sheet-1', { stats: { dexterity: 12 } });

    expect(payload).toEqual({ modifiers: { dexterityMod: 3 } });
    expect(fake.lastRequest?.method).toBe('PATCH');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/sheet-1`);
    expect(fake.lastRequest?.body).toBe(JSON.stringify({ data: { stats: { dexterity: 12 } } }));
    expect(Object.keys(JSON.parse(fake.lastRequest?.body ?? '{}'))).toEqual(['data']);
  });

  it('Edge: the caller’s change set is never mutated by an update (API-06)', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const changeSet = { stats: { dexterity: 12 } };
    const snapshot = JSON.stringify(changeSet);

    await clientWith(fake).patchSheet('sheet-1', changeSet);

    expect(JSON.stringify(changeSet)).toBe(snapshot);
  });

  it('Edge: a path identifier is percent-encoded and otherwise passed through verbatim (API-01)', async () => {
    const fake = createFakeTransport(successOutcome(sheetPayload));
    await clientWith(fake).getSheet('a/b c?d');

    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/${encodeURIComponent('a/b c?d')}`);
  });

  it('Edge: no read carries a request body, so none carries a content type (API-05, API-07)', async () => {
    const fake = createFakeTransport(successOutcome({ status: 'ok' }));

    await clientWith(fake).health();

    expect(fake.lastRequest?.body).toBeUndefined();
    expect(fake.lastRequest?.headers).toEqual({});
  });

  it('Error: a failure envelope raises a domain failure carrying the code and message (API-03)', async () => {
    const fake = createFakeTransport(errorOutcome('NOT_FOUND', 'sheet gone', 404));

    await expect(clientWith(fake).getSheet('missing')).rejects.toMatchObject({
      name: 'DomainFailure',
      code: 'NOT_FOUND',
      status: 404,
      message: 'sheet gone',
    });
  });

  it('Error: an unreadable success body raises a defensive failure rather than an empty success (API-04)', async () => {
    const fake = createFakeTransport({ status: 200, body: 'not json' });

    await expect(clientWith(fake).listTemplates()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a 2xx without the data envelope raises a defensive failure (API-04)', async () => {
    const fake = createFakeTransport({ status: 200, body: JSON.stringify({ ok: true }) });

    await expect(clientWith(fake).health()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a 2xx whose payload does not match the model raises a defensive failure (API-04, API-08)', async () => {
    const fake = createFakeTransport(successOutcome({ id: 'fantasy_v1' }));

    await expect(clientWith(fake).getTemplate('fantasy_v1')).rejects.toBeInstanceOf(
      DefensiveParseFailure,
    );
  });

  it('Error: a success status with an empty body fails defensively instead of succeeding (API-04)', async () => {
    const fake = createFakeTransport({ status: 200, body: '' });

    await expect(clientWith(fake).health()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: the six operations never use a transport other than the injected one (API-07)', async () => {
    const fakes = [
      createFakeTransport(successOutcome({ status: 'ok' })),
      createFakeTransport(successOutcome([templatePayload])),
      createFakeTransport(successOutcome(templatePayload)),
      createFakeTransport(successOutcome(sheetPayload, 201)),
      createFakeTransport(successOutcome(sheetPayload)),
      createFakeTransport(successOutcome({})),
    ];

    const [health, list, detail, create, get, patch] = fakes.map((fake) => clientWith(fake));

    await health.health();
    await list.listTemplates();
    await detail.getTemplate('fantasy_v1');
    await create.createSheet({ templateId: 'fantasy_v1', sheetName: 'Aragorn' });
    await get.getSheet('sheet-1');
    await patch.patchSheet('sheet-1', { stats: { dexterity: 12 } });

    for (const fake of fakes) {
      expect(fake.attempts).toBe(1);
    }
  });

  it('Error: a domain failure is distinct from a defensive failure (API-03, API-04)', async () => {
    const failed = createFakeTransport(errorOutcome('INTERNAL_ERROR', 'boom', 500));
    const drifted = createFakeTransport({ status: 200, body: '{' });

    const domainError = await clientWith(failed)
      .health()
      .catch((error: unknown) => error);
    const parseError = await clientWith(drifted)
      .health()
      .catch((error: unknown) => error);

    expect(domainError).toBeInstanceOf(DomainFailure);
    expect(domainError).not.toBeInstanceOf(DefensiveParseFailure);
    expect(parseError).toBeInstanceOf(DefensiveParseFailure);
    expect(parseError).not.toBeInstanceOf(DomainFailure);
  });
});
