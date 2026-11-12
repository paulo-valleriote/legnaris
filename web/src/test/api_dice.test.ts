import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';
const SHEET = '01a0f967-f42b-710e-851f-8f293a634921';

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

const sheetPayload = (extra: Record<string, unknown> = {}) => ({
  id: SHEET,
  template_id: 'fantasy_v1',
  name: 'Aria',
  data: {},
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
  ...extra,
});

describe('patchSheet with a Dice block (ADR 0006)', () => {
  it('Happy path: PATCHes the delta and the block together and decodes the cascade', async () => {
    const fake = createFakeTransport(successOutcome({ attrs: { mod: 2 } }));

    const cascade = await clientWith(fake).patchSheet(
      SHEET,
      { attrs: { str: 14 } },
      { section: { attrs: '1d20' }, row: { 'attrs.init': '2d6' } },
    );

    expect(cascade).toEqual({ attrs: { mod: 2 } });
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('PATCH');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/${SHEET}`);
    expect(fake.lastRequest?.credentials).toBe('omit');
    expect(fake.lastRequest?.body).toBe(
      '{"data":{"attrs":{"str":14}},"dice":{"section":{"attrs":"1d20"},"row":{"attrs.init":"2d6"}}}',
    );
  });

  it('Edge: a save of only a block leaves the empty delta out, so the server skips the recalculation', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await clientWith(fake).patchSheet(SHEET, {}, { section: {}, row: { 'attrs.init': '2d6' } });

    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{},"row":{"attrs.init":"2d6"}}}');
  });

  it('Edge: without a block the body is the delta alone, never a dice key', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await clientWith(fake).patchSheet(SHEET, { attrs: { str: 14 } });

    expect(fake.lastRequest?.body).toBe('{"data":{"attrs":{"str":14}}}');
  });

  it('Edge: a null entry travels as an explicit "no dice"', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await clientWith(fake).patchSheet(SHEET, {}, { section: { attrs: null }, row: {} });

    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{"attrs":null},"row":{}}}');
  });

  it('Edge: an empty block is sent as two empty maps, never omitted, so it clears every override', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await clientWith(fake).patchSheet(SHEET, {}, { section: {}, row: {} });

    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{},"row":{}}}');
  });

  it('Error: a 400 INVALID_REQUEST is a DomainFailure of kind invalidEdit', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'bad grammar', 400));

    const failure = await clientWith(fake)
      .patchSheet(SHEET, {}, { section: {}, row: { 'attrs.init': 'd' } })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe('invalidEdit');
    expect(fake.attempts).toBe(1);
  });

  it('Edge: a write never repeats after a transport failure', async () => {
    const fake = createFakeTransport({ failure: new Error('offline') });

    await expect(
      clientWith(fake).patchSheet(SHEET, {}, { section: {}, row: {} }),
    ).rejects.toBeDefined();
    expect(fake.attempts).toBe(1);
  });

  it('Edge: the client has no separate dice write any more', () => {
    expect('setDice' in clientWith(createFakeTransport(successOutcome({})))).toBe(false);
  });
});

describe('dice on the sheet payload', () => {
  it('Happy path: a sheet carries its dice block as stored', () => {
    const sheet = parseSheet(
      sheetPayload({ dice: { section: { attrs: '1d20' }, row: { 'attrs.init': null } } }),
    );

    expect(sheet.dice).toEqual({ section: { attrs: '1d20' }, row: { 'attrs.init': null } });
  });

  it('Edge: a sheet without a dice block reads as an empty block, not as missing', () => {
    expect(parseSheet(sheetPayload()).dice).toEqual({ section: {}, row: {} });
  });

  it('Edge: an empty-string override is kept verbatim for the UI to judge', () => {
    const sheet = parseSheet(sheetPayload({ dice: { section: {}, row: { 'attrs.init': '' } } }));

    expect(sheet.dice?.row).toEqual({ 'attrs.init': '' });
  });
});

describe('dice defaults on the template payload', () => {
  const templatePayload = (extra: Record<string, unknown> = {}) => ({
    id: 'fantasy_v1',
    name: 'Fantasy',
    default: true,
    fields: {},
    ...extra,
  });

  it('Happy path: the Template dice defaults are decoded', () => {
    const template = parseTemplateDetail(
      templatePayload({
        dice: { section: { attrs: '1d20' }, row: { 'attrs.init': '1d20+@.mod' } },
      }),
    );

    expect(template.dice).toEqual({
      section: { attrs: '1d20' },
      row: { 'attrs.init': '1d20+@.mod' },
    });
  });

  it('Edge: a template without dice has no defaults, and an empty side is an empty map', () => {
    expect(parseTemplateDetail(templatePayload()).dice).toBeUndefined();
    expect(parseTemplateDetail(templatePayload({ dice: { row: { a: '1d4' } } })).dice).toEqual({
      section: {},
      row: { a: '1d4' },
    });
  });
});
