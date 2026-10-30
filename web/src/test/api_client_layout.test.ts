import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';
const SHEET = '01a0f967-f42b-710e-851f-8f293a634921';

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

describe('setLayout', () => {
  it('Happy path: PUTs {"layout": wire} to the layout path and decodes the stored layout', async () => {
    const fake = createFakeTransport(successOutcome({ layout: { pinned: ['a.x'] } }));
    const stored = await clientWith(fake).setLayout(SHEET, {
      sectionOrder: [],
      pinned: ['a.x'],
    });

    expect(stored).toEqual({ sectionOrder: [], pinned: ['a.x'] });
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('PUT');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/${SHEET}/layout`);
    expect(fake.lastRequest?.credentials).toBe('omit');
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ layout: { pinned: ['a.x'] } });
  });

  it('Edge: null sends {"layout": null} and resolves null for a reset', async () => {
    const fake = createFakeTransport(successOutcome({ layout: null }));

    await expect(clientWith(fake).setLayout(SHEET, null)).resolves.toBeNull();
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ layout: null });
  });

  it('Edge: the body carries only the lists that are set and never row_order', async () => {
    const fake = createFakeTransport(successOutcome({ layout: { section_order: ['b', 'a'] } }));

    await clientWith(fake).setLayout(SHEET, { sectionOrder: ['b', 'a'], pinned: [] });

    expect(fake.lastRequest?.body).toBe('{"layout":{"section_order":["b","a"]}}');
  });

  it('Edge: the sheet id is path-encoded', async () => {
    const fake = createFakeTransport(successOutcome({ layout: null }));

    await clientWith(fake).setLayout('a/b?c', null);

    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/a%2Fb%3Fc/layout`);
  });

  it('Edge: a write is never repeated when the transport fails', async () => {
    const fake = createFakeTransport({ failure: new Error('network') });

    await expect(clientWith(fake).setLayout(SHEET, null)).rejects.toMatchObject({
      kind: 'offline',
    });
    expect(fake.attempts).toBe(1);
  });

  it.each([
    ['404 NOT_FOUND', 'NOT_FOUND', 404, 'notFound'],
    ['400 INVALID_REQUEST', 'INVALID_REQUEST', 400, 'invalidEdit'],
    ['400 INVALID_JSON', 'INVALID_JSON', 400, 'clientFault'],
    ['401 UNAUTHORIZED', 'UNAUTHORIZED', 401, 'unauthorized'],
  ])('Error: %s maps to the %s kind', async (_name, code, status, kind) => {
    const fake = createFakeTransport(errorOutcome(code, 'refused', status));

    const failure = await clientWith(fake)
      .setLayout(SHEET, null)
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe(kind);
    expect(fake.attempts).toBe(1);
  });

  it('Error: an answer without a layout key is contract drift', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await expect(clientWith(fake).setLayout(SHEET, null)).rejects.toBeInstanceOf(
      DefensiveParseFailure,
    );
  });
});
