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

describe('setDice', () => {
  it('Happy path: PUTs {"dice": block} to the dice path and decodes the stored block', async () => {
    const fake = createFakeTransport(
      successOutcome({ dice: { section: { attrs: '1d20' }, row: { 'attrs.init': '2d6' } } }),
    );

    const stored = await clientWith(fake).setDice(SHEET, {
      section: { attrs: '1d20' },
      row: { 'attrs.init': '2d6' },
    });

    expect(stored).toEqual({ section: { attrs: '1d20' }, row: { 'attrs.init': '2d6' } });
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('PUT');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/${SHEET}/dice`);
    expect(fake.lastRequest?.credentials).toBe('omit');
    expect(fake.lastRequest?.body).toBe(
      '{"dice":{"section":{"attrs":"1d20"},"row":{"attrs.init":"2d6"}}}',
    );
  });

  it('Edge: a null entry travels as an explicit "no dice" and is kept in the stored block', async () => {
    const fake = createFakeTransport(
      successOutcome({ dice: { section: { attrs: null }, row: {} } }),
    );

    const stored = await clientWith(fake).setDice(SHEET, { section: { attrs: null }, row: {} });

    expect(stored.section).toEqual({ attrs: null });
    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{"attrs":null},"row":{}}}');
  });

  it('Edge: an empty block is sent as two empty maps, never omitted', async () => {
    const fake = createFakeTransport(successOutcome({ dice: { section: {}, row: {} } }));

    await clientWith(fake).setDice(SHEET, { section: {}, row: {} });

    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{},"row":{}}}');
  });

  it('Error: a 400 INVALID_REQUEST is a DomainFailure of kind invalidEdit', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'bad grammar', 400));

    const failure = await clientWith(fake)
      .setDice(SHEET, { section: {}, row: { 'attrs.init': 'd' } })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect((failure as DomainFailure).kind).toBe('invalidEdit');
    expect(fake.attempts).toBe(1);
  });

  it('Edge: a write never repeats after a transport failure', async () => {
    const fake = createFakeTransport({ failure: new Error('offline') });

    await expect(clientWith(fake).setDice(SHEET, { section: {}, row: {} })).rejects.toBeDefined();
    expect(fake.attempts).toBe(1);
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
