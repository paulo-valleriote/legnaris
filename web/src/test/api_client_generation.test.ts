import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { parseGeneration } from '../api/models/generation';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';
const generationId = '01a0f967-f42b-710e-851f-8f293a634921';

const generationPayload = {
  id: generationId,
  status: 'succeeded',
  kind: 'character',
  template_id: 'fantasy_v1',
  values: { stats: { strength: 12 } },
  created_at: '2026-11-14T10:00:00Z',
};

function clientWith(...outcomes: Parameters<typeof createFakeTransport>) {
  const fake = createFakeTransport(...outcomes);
  return { fake, client: createApiClient({ baseUrl, transport: fake.transport }) };
}

describe('api client generation operations', () => {
  it('Happy path: createGeneration posts the concept and returns the 202 identifier (LGN-196)', async () => {
    const { fake, client } = clientWith(successOutcome({ id: generationId }, 202));

    const id = await client.createGeneration({
      templateId: 'fantasy_v1',
      kind: 'character',
      concept: 'A wandering ranger',
    });

    expect(id).toBe(generationId);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/generations`);
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      template_id: 'fantasy_v1',
      kind: 'character',
      concept: 'A wandering ranger',
    });
  });

  it('Error: createGeneration maps a 403 PLAN_UPGRADE_REQUIRED to a domain failure with its code', async () => {
    const { client } = clientWith(errorOutcome('PLAN_UPGRADE_REQUIRED', 'no allowance left', 403));

    const failure = await client
      .createGeneration({ templateId: 'fantasy_v1', kind: 'character', concept: 'x' })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe('planUpgradeRequired');
    expect((failure as DomainFailure).code).toBe('PLAN_UPGRADE_REQUIRED');
  });

  it('Error: createGeneration keeps the Retry-After wait of a 429 QUOTA_EXCEEDED', async () => {
    const response = errorOutcome('QUOTA_EXCEEDED', 'daily quota spent', 429);
    const { client } = clientWith({ ...response, retryAfter: '90' });

    const failure = await client
      .createGeneration({ templateId: 'fantasy_v1', kind: 'character', concept: 'x' })
      .catch((error: unknown) => error);

    expect((failure as DomainFailure).kind).toBe('quotaExceeded');
    expect((failure as DomainFailure).retryAfterMs).toBe(90_000);
  });

  it('Happy path: getGeneration reads GET /generations/:id and decodes the status (LGN-196)', async () => {
    const { fake, client } = clientWith(successOutcome(generationPayload));

    const generation = await client.getGeneration(generationId);

    expect(generation.status).toBe('succeeded');
    expect(generation.templateId).toBe('fantasy_v1');
    expect(generation.values).toEqual({ stats: { strength: 12 } });
    expect(fake.lastRequest?.method).toBe('GET');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/generations/${generationId}`);
  });

  it('Edge: getGeneration decodes a failed generation with its error code and no values', async () => {
    const { client } = clientWith(
      successOutcome({
        ...generationPayload,
        status: 'failed',
        values: undefined,
        error_code: 'invalid_output',
      }),
    );

    const generation = await client.getGeneration(generationId);

    expect(generation.status).toBe('failed');
    expect(generation.errorCode).toBe('invalid_output');
    expect(generation.values).toBeUndefined();
  });

  it('Error: getGeneration rejects an unknown status as drift, never as a value', async () => {
    const { client } = clientWith(successOutcome({ ...generationPayload, status: 'exploded' }));

    await expect(client.getGeneration(generationId)).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Happy path: acceptGeneration posts the name and values and returns the sheet id (LGN-196)', async () => {
    const { fake, client } = clientWith(successOutcome({ sheet_id: 'sheet-1' }, 201));

    const result = await client.acceptGeneration(generationId, {
      sheetName: 'Aragorn',
      values: { stats: { strength: 14 } },
    });

    expect(result).toEqual({ sheetId: 'sheet-1' });
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/generations/${generationId}/accept`);
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      sheet_name: 'Aragorn',
      values: { stats: { strength: 14 } },
    });
  });

  it('Edge: acceptGeneration treats a repeat accept (200) the same as the first (201)', async () => {
    const { client } = clientWith(successOutcome({ sheet_id: 'sheet-1' }, 200));

    await expect(
      client.acceptGeneration(generationId, { sheetName: 'Aragorn', values: {} }),
    ).resolves.toEqual({ sheetId: 'sheet-1' });
  });

  it('Happy path: previewTemplate posts the delta to the template preview and returns the cascade (LGN-196)', async () => {
    const cascade = { stats: { modifier: 1 } };
    const { fake, client } = clientWith(successOutcome(cascade));

    const result = await client.previewTemplate('fantasy_v1', { stats: { strength: 14 } });

    expect(result).toEqual(cascade);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/templates/fantasy_v1/preview`);
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      data: { stats: { strength: 14 } },
    });
  });

  it('Happy path: grantLlmConsent puts the disclosure version and accepts a 204 (LGN-196)', async () => {
    const { fake, client } = clientWith({ status: 204, body: '' });

    await client.grantLlmConsent('1');

    expect(fake.lastRequest?.method).toBe('PUT');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/me/llm-consent`);
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({ version: '1' });
  });

  it('Edge: a write is never repeated on a transport failure (API-33)', async () => {
    const { fake, client } = clientWith({ failure: new Error('network down') });

    await expect(
      client.createGeneration({ templateId: 'fantasy_v1', kind: 'character', concept: 'x' }),
    ).rejects.toBeInstanceOf(DomainFailure);
    expect(fake.attempts).toBe(1);
  });
});

describe('parseGeneration', () => {
  it('Edge: rejects a payload without a string id', () => {
    expect(() => parseGeneration({ ...generationPayload, id: 7 })).toThrow(DefensiveParseFailure);
  });

  it('Edge: a non-object payload is drift', () => {
    expect(() => parseGeneration(null)).toThrow(DefensiveParseFailure);
  });
});
