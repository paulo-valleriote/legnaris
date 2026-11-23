import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';
const SHEET = '01a0f967-f42b-710e-851f-8f293a634921';
const REQUEST = '01900000-0000-7000-8000-0000000000c1';

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

describe('resetSheet', () => {
  it('Happy path: POSTs the request id to the reset path and decodes the cascade', async () => {
    const fake = createFakeTransport(successOutcome({ stats: { level: 4 } }));
    const cascade = await clientWith(fake).resetSheet(SHEET, 'rest', REQUEST);

    expect(cascade).toEqual({ stats: { level: 4 } });
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/${SHEET}/resets/rest`);
    expect(fake.lastRequest?.credentials).toBe('omit');
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ request_id: REQUEST });
  });

  it('Edge: both the sheet id and the action id are path-encoded', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await clientWith(fake).resetSheet('a/b?c', 'rest/now', REQUEST);

    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/a%2Fb%3Fc/resets/rest%2Fnow`);
  });

  it('Edge: an empty cascade decodes to an empty object', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await expect(clientWith(fake).resetSheet(SHEET, 'rest', REQUEST)).resolves.toEqual({});
  });

  it('Edge: a reset is never repeated when the transport fails', async () => {
    const fake = createFakeTransport({ failure: new Error('network') });

    await expect(clientWith(fake).resetSheet(SHEET, 'rest', REQUEST)).rejects.toMatchObject({
      kind: 'offline',
    });
    expect(fake.attempts).toBe(1);
  });

  it('Edge: an answer that is not an object is contract drift', async () => {
    const fake = createFakeTransport(successOutcome([1, 2]));

    await expect(clientWith(fake).resetSheet(SHEET, 'rest', REQUEST)).rejects.toBeInstanceOf(
      DefensiveParseFailure,
    );
  });

  it.each([
    ['404 NOT_FOUND', 'NOT_FOUND', 404, 'notFound'],
    ['400 INVALID_REQUEST', 'INVALID_REQUEST', 400, 'invalidEdit'],
    ['409 CONFLICT', 'CONFLICT', 409, 'conflict'],
    ['401 UNAUTHORIZED', 'UNAUTHORIZED', 401, 'unauthorized'],
  ])('Error: %s maps to the %s kind', async (_name, code, status, kind) => {
    const fake = createFakeTransport(errorOutcome(code, 'refused', status));

    const failure = await clientWith(fake)
      .resetSheet(SHEET, 'rest', REQUEST)
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe(kind);
    expect(fake.attempts).toBe(1);
  });
});
