import { describe, expect, it } from 'vitest';
import { parseGeneration } from '../api/models/generation';
import { SHEET_KINDS } from '../api/models/plan';
import { DefensiveParseFailure } from '../api/errors';

const base = {
  id: '01a0f967-f42b-710e-851f-8f293a634921',
  kind: 'character',
  template_id: 'fantasy_v1',
  created_at: '2026-11-14T10:00:00Z',
};

describe('parseGeneration suggested_name (LGN-233)', () => {
  it('Happy path: a suggested name is decoded as the Sheet name suggestion', () => {
    const generation = parseGeneration({
      ...base,
      status: 'succeeded',
      values: { identity: { characterName: 'Aragorn' } },
      suggested_name: 'Aragorn',
    });

    expect(generation.suggestedName).toBe('Aragorn');
  });

  it('Edge: an absent suggested_name decodes to undefined', () => {
    const generation = parseGeneration({ ...base, status: 'succeeded', values: {} });

    expect(generation.suggestedName).toBeUndefined();
  });

  it('Edge: a null suggested_name decodes to undefined', () => {
    const generation = parseGeneration({ ...base, status: 'queued', suggested_name: null });

    expect(generation.suggestedName).toBeUndefined();
  });

  it('Edge: an empty suggested_name decodes to undefined', () => {
    const generation = parseGeneration({
      ...base,
      status: 'succeeded',
      values: {},
      suggested_name: '',
    });

    expect(generation.suggestedName).toBeUndefined();
  });

  it('Edge: a whitespace-only suggested_name decodes to undefined', () => {
    const generation = parseGeneration({
      ...base,
      status: 'succeeded',
      values: {},
      suggested_name: ' \t ',
    });

    expect(generation.suggestedName).toBeUndefined();
  });

  it('Error: a non-string suggested_name is contract drift', () => {
    expect(() =>
      parseGeneration({ ...base, status: 'succeeded', values: {}, suggested_name: 42 }),
    ).toThrow(DefensiveParseFailure);
  });
});

describe('parseGeneration kind (LGN-187)', () => {
  it('Happy path: every known Sheet kind decodes to the same kind', () => {
    for (const kind of SHEET_KINDS) {
      const generation = parseGeneration({ ...base, kind, status: 'queued' });

      expect(generation.kind).toBe(kind);
    }
  });

  it('Edge: an unknown kind is contract drift, never an opaque string', () => {
    expect(() => parseGeneration({ ...base, kind: 'monster', status: 'queued' })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Error: a missing kind is contract drift', () => {
    const { id, template_id, created_at } = base;

    expect(() => parseGeneration({ id, template_id, created_at, status: 'queued' })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Error: a non-string kind is contract drift', () => {
    expect(() => parseGeneration({ ...base, kind: 3, status: 'queued' })).toThrow(
      DefensiveParseFailure,
    );
  });
});
