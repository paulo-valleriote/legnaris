import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure } from '../api/errors';
import {
  isEmptyLayout,
  isLayout,
  layoutToWire,
  parseLayout,
  parseLayoutResponse,
  parseOptionalLayout,
} from '../api/models/layout';
import { parseSheet } from '../api/models/sheet';
import { sheetPayload } from './sheet_fixtures';

describe('parseLayout', () => {
  it('Happy path: decodes the two lists', () => {
    expect(parseLayout({ section_order: ['b', 'a'], pinned: ['a.y', 'a.x'] })).toEqual({
      sectionOrder: ['b', 'a'],
      pinned: ['a.y', 'a.x'],
    });
  });

  it('Edge: the retired row_order key is ignored, never decoded', () => {
    const decoded = parseLayout({ section_order: ['b'], row_order: { a: ['a.x'] } });

    expect(decoded).toEqual({ sectionOrder: ['b'], pinned: [] });
    expect('rowOrder' in decoded).toBe(false);
  });

  it('Edge: duplicate and unknown keys are kept as sent; resolution is the engine job', () => {
    expect(parseLayout({ pinned: ['a', 'a', 'ghost'] }).pinned).toEqual(['a', 'a', 'ghost']);
  });

  it('Edge: omitted and null lists decode as empty, as the backend omits empty lists', () => {
    const empty = { sectionOrder: [], pinned: [] };

    expect(parseLayout({})).toEqual(empty);
    expect(parseLayout({ section_order: null, pinned: null })).toEqual(empty);
  });

  it('Edge: keys named like prototype members decode as plain strings', () => {
    expect(parseLayout({ section_order: ['__proto__'], pinned: ['constructor'] })).toEqual({
      sectionOrder: ['__proto__'],
      pinned: ['constructor'],
    });
  });

  it.each([
    ['null', null],
    ['an array', []],
    ['a string', 'layout'],
    ['section_order that is not a list', { section_order: 'a' }],
    ['a non-string section key', { section_order: ['a', 1] }],
    ['pinned that is not a list', { pinned: { a: 1 } }],
    ['a non-string pinned key', { pinned: [null] }],
  ])('Error: %s is contract drift', (_name, payload) => {
    expect(() => parseLayout(payload)).toThrow(DefensiveParseFailure);
  });
});

describe('parseOptionalLayout and the sheet model', () => {
  it('Edge: absent and null mean no layout', () => {
    expect(parseOptionalLayout(undefined)).toBeUndefined();
    expect(parseOptionalLayout(null)).toBeUndefined();
    expect('layout' in parseSheet(sheetPayload())).toBe(false);
    expect('layout' in parseSheet(sheetPayload({ layout: null }))).toBe(false);
  });

  it('Happy path: a sheet carries its layout when set', () => {
    const sheet = parseSheet(sheetPayload({ layout: { pinned: ['identity.level'] } }));

    expect(sheet.layout).toEqual({ sectionOrder: [], pinned: ['identity.level'] });
  });

  it('Error: a malformed layout makes the sheet payload drift', () => {
    expect(() => parseSheet(sheetPayload({ layout: { pinned: 'identity.level' } }))).toThrow(
      DefensiveParseFailure,
    );
    expect(() => parseSheet(sheetPayload({ layout: 'x' }))).toThrow(DefensiveParseFailure);
  });
});

describe('layoutToWire', () => {
  it('Happy path: serializes to snake_case', () => {
    expect(layoutToWire({ sectionOrder: ['b'], pinned: ['a.x'] })).toEqual({
      section_order: ['b'],
      pinned: ['a.x'],
    });
  });

  it('Edge: omits every empty list and never emits row_order', () => {
    expect(layoutToWire({ sectionOrder: [], pinned: [] })).toEqual({});
    expect(layoutToWire({ sectionOrder: [], pinned: ['p'] })).toEqual({ pinned: ['p'] });
    expect(JSON.stringify(layoutToWire({ sectionOrder: ['b'], pinned: ['p'] }))).not.toContain(
      'row_order',
    );
  });

  it('Edge: a decoded layout survives a wire round trip', () => {
    const layout = parseLayout({ section_order: ['b', 'a'], pinned: ['a.y'] });

    expect(parseLayout(layoutToWire(layout))).toEqual(layout);
  });
});

describe('isLayout and isEmptyLayout', () => {
  it('Edge: isLayout accepts only a well-formed model', () => {
    expect(isLayout({ sectionOrder: [], pinned: [] })).toBe(true);
    expect(isLayout({ sectionOrder: ['a'], pinned: ['a.x'] })).toBe(true);
    expect(isLayout(null)).toBe(false);
    expect(isLayout([])).toBe(false);
    expect(isLayout({ sectionOrder: [] })).toBe(false);
    expect(isLayout({ pinned: [] })).toBe(false);
    expect(isLayout({ sectionOrder: [1], pinned: [] })).toBe(false);
    expect(isLayout({ sectionOrder: [], pinned: 'x' })).toBe(false);
  });

  it('Edge: empty means no ordering at all', () => {
    expect(isEmptyLayout(undefined)).toBe(true);
    expect(isEmptyLayout({ sectionOrder: [], pinned: [] })).toBe(true);
    expect(isEmptyLayout({ sectionOrder: [], pinned: ['a'] })).toBe(false);
    expect(isEmptyLayout({ sectionOrder: ['a'], pinned: [] })).toBe(false);
  });
});

describe('parseLayoutResponse', () => {
  it('Happy path: decodes the stored layout and a reset', () => {
    expect(parseLayoutResponse({ layout: { pinned: ['a'] } })).toEqual({
      sectionOrder: [],
      pinned: ['a'],
    });
    expect(parseLayoutResponse({ layout: null })).toBeNull();
  });

  it.each([
    ['a missing layout key', {}],
    ['a non-object payload', 'ok'],
    ['null', null],
    ['a malformed layout', { layout: { pinned: 1 } }],
  ])('Error: %s is contract drift', (_name, payload) => {
    expect(() => parseLayoutResponse(payload)).toThrow(DefensiveParseFailure);
  });
});
