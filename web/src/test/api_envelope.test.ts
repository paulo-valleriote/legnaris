import { describe, expect, it, vi } from 'vitest';
import { decodeEnvelope } from '../api/envelope';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { errorOutcome, successOutcome } from './fake_transport';

const identity = (data: unknown) => data;

describe('envelope decoding', () => {
  it('Happy path: a 2xx body decodes its data through the operation callback (API-02)', () => {
    const decoded = decodeEnvelope({ ...successOutcome({ status: 'healthy' }), decode: identity });

    expect(decoded).toEqual({ status: 'healthy' });
  });

  it('Happy path: the per-operation callback receives exactly the data value (API-02)', () => {
    const decode = vi.fn().mockReturnValue('decoded');

    const decoded = decodeEnvelope({ ...successOutcome([{ id: 'fantasy_v1' }]), decode });

    expect(decode).toHaveBeenCalledWith([{ id: 'fantasy_v1' }]);
    expect(decoded).toBe('decoded');
  });

  it('Happy path: a failure body raises a domain failure carrying status, code and message (API-03, API-24)', () => {
    expect.assertions(5);

    try {
      decodeEnvelope({ ...errorOutcome('NOT_FOUND', 'sheet not found', 404), decode: identity });
    } catch (error) {
      expect(error).toBeInstanceOf(DomainFailure);
      const failure = error as DomainFailure;
      expect(failure.status).toBe(404);
      expect(failure.code).toBe('NOT_FOUND');
      expect(failure.message).toBe('sheet not found');
      expect(failure.kind).toBe('unknown');
    }
  });

  it('Edge: `data: null` is a successful payload, not a missing one (API-02)', () => {
    const decoded = decodeEnvelope({
      status: 200,
      body: '{"data":null}',
      decode: () => 'decoded-null',
    });

    expect(decoded).toBe('decoded-null');
  });

  it('Edge: `data: false` and `data: 0` are payloads too, so a falsy value is not read as absence (API-04)', () => {
    const falsy = [false, 0, ''].map((value) =>
      decodeEnvelope({ status: 200, body: `{"data":${JSON.stringify(value)}}`, decode: identity }),
    );

    expect(falsy).toEqual([false, 0, '']);
  });

  it('Edge: an array payload decodes as an array (API-02)', () => {
    const decoded = decodeEnvelope({ ...successOutcome([1, 2, 3]), decode: identity });

    expect(decoded).toEqual([1, 2, 3]);
  });

  it('Error: an empty body on a 2xx is a defensive parse failure, never an empty success (API-04)', () => {
    expect(() => decodeEnvelope({ status: 200, body: '', decode: identity })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Error: invalid JSON on a 2xx is a defensive parse failure (API-04)', () => {
    expect(() => decodeEnvelope({ status: 200, body: '{not json', decode: identity })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Error: a 2xx without the data key is a defensive parse failure (API-04)', () => {
    expect(() => decodeEnvelope({ status: 200, body: '{"items":[]}', decode: identity })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Error: a 2xx whose body is not an object is a defensive parse failure (API-04)', () => {
    expect(() => decodeEnvelope({ status: 200, body: '"ok"', decode: identity })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Error: a failure body without error.code is a defensive parse failure (API-22)', () => {
    expect(() =>
      decodeEnvelope({ status: 500, body: '{"error":{"message":"boom"}}', decode: identity }),
    ).toThrow(DefensiveParseFailure);
  });

  it('Error: a failure body with an empty error.code is a defensive parse failure (API-22)', () => {
    expect(() =>
      decodeEnvelope({
        status: 400,
        body: '{"error":{"code":"","message":"boom"}}',
        decode: identity,
      }),
    ).toThrow(DefensiveParseFailure);
  });

  it('Error: a failure body with a non-string error.code is a defensive parse failure (API-22)', () => {
    expect(() =>
      decodeEnvelope({
        status: 400,
        body: '{"error":{"code":400,"message":"boom"}}',
        decode: identity,
      }),
    ).toThrow(DefensiveParseFailure);
  });

  it('Error: invalid JSON on a failure is a defensive parse failure, never a domain failure (API-22)', () => {
    expect.assertions(2);

    try {
      decodeEnvelope({ status: 500, body: 'gateway timeout', decode: identity });
    } catch (error) {
      expect(error).toBeInstanceOf(DefensiveParseFailure);
      expect(error).not.toBeInstanceOf(DomainFailure);
    }
  });

  it('Edge: a missing error.message becomes an empty string, keeping the code authoritative (API-24)', () => {
    expect.assertions(2);

    try {
      decodeEnvelope({
        status: 500,
        body: '{"error":{"code":"INTERNAL_ERROR"}}',
        decode: identity,
      });
    } catch (error) {
      const failure = error as DomainFailure;
      expect(failure.code).toBe('INTERNAL_ERROR');
      expect(failure.message).toBe('');
    }
  });

  it('Edge: the status travels on the defensive parse failure so drift is attributable (API-04)', () => {
    expect.assertions(1);

    try {
      decodeEnvelope({ status: 200, body: '', decode: identity });
    } catch (error) {
      expect((error as DefensiveParseFailure).status).toBe(200);
    }
  });

  it('Edge: a 204 with no body is a defensive parse failure, as the contract requires a body (API-04)', () => {
    expect(() => decodeEnvelope({ status: 204, body: '', decode: identity })).toThrow(
      DefensiveParseFailure,
    );
  });
});
