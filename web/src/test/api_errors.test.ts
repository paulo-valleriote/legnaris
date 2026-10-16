import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure, DomainFailure, PathFailure } from '../api/errors';

describe('client error shapes', () => {
  describe('DomainFailure', () => {
    it('Happy path: carries the status, raw code, server message and kind (API-03, API-24)', () => {
      const failure = new DomainFailure({
        kind: 'notFound',
        status: 404,
        code: 'NOT_FOUND',
        message: 'sheet not found',
      });

      expect(failure).toBeInstanceOf(Error);
      expect(failure.kind).toBe('notFound');
      expect(failure.status).toBe(404);
      expect(failure.code).toBe('NOT_FOUND');
      expect(failure.message).toBe('sheet not found');
    });

    it('Edge: an absent status stays absent for a transport failure (API-20)', () => {
      const failure = new DomainFailure({ kind: 'offline', message: 'transport failure' });

      expect(failure.status).toBeUndefined();
      expect(failure.code).toBeUndefined();
    });

    it('Edge: a non-string server message becomes an empty string (API-24)', () => {
      const failure = new DomainFailure({
        kind: 'serverError',
        status: 500,
        message: 42 as unknown,
      });

      expect(failure.message).toBe('');
    });

    it('Edge: a non-string raw code becomes absent rather than a coerced string (API-24)', () => {
      const failure = new DomainFailure({ kind: 'unknown', status: 409, code: 409 as unknown });

      expect(failure.code).toBeUndefined();
    });
  });

  describe('DefensiveParseFailure', () => {
    it('Happy path: carries the reason and the status it was seen at (API-04, API-22)', () => {
      const failure = new DefensiveParseFailure({ reason: 'missing data envelope', status: 200 });

      expect(failure).toBeInstanceOf(Error);
      expect(failure.reason).toBe('missing data envelope');
      expect(failure.status).toBe(200);
      expect(failure.message).toContain('missing data envelope');
    });

    it('Edge: a defensive failure is not a domain failure, so the two are distinguishable (API-22)', () => {
      const parse = new DefensiveParseFailure({ reason: 'invalid JSON' });
      const domain = new DomainFailure({ kind: 'unknown', message: 'invalid JSON' });

      expect(parse).not.toBeInstanceOf(DomainFailure);
      expect(domain).not.toBeInstanceOf(DefensiveParseFailure);
      expect(parse.status).toBeUndefined();
    });
  });

  describe('PathFailure', () => {
    it('Happy path: carries the offending path (API-14)', () => {
      const failure = new PathFailure('stats..dexterity');

      expect(failure).toBeInstanceOf(Error);
      expect(failure.path).toBe('stats..dexterity');
      expect(failure.message).toContain('stats..dexterity');
    });

    it('Edge: a path failure is neither a domain failure nor a parse failure (API-14)', () => {
      const failure = new PathFailure('');

      expect(failure).not.toBeInstanceOf(DomainFailure);
      expect(failure).not.toBeInstanceOf(DefensiveParseFailure);
    });
  });
});
