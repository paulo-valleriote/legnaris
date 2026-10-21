import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import { apperrKind } from '../api/error_mapper';
import { createFakeTransport, errorOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

describe('failure states out of the client', () => {
  it('Happy path: a failed response leaves the client with the mapped kind, status, code and message (API-21, API-24)', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'the edit was refused', 400));

    const failure = await clientWith(fake)
      .patchSheet('sheet-1', { stats: { dexterity: 12 } })
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({
      kind: 'invalidEdit',
      status: 400,
      code: 'INVALID_REQUEST',
      message: 'the edit was refused',
    });
  });

  it('Edge: a failure with an unmapped code keeps the generic kind, the raw code and the message (API-21)', async () => {
    const fake = createFakeTransport(errorOutcome('SHEET_LIMIT_REACHED', 'already exists', 409));

    const failure = await clientWith(fake)
      .getSheet('sheet-1')
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({
      kind: 'unknown',
      status: 409,
      code: 'SHEET_LIMIT_REACHED',
      message: 'already exists',
    });
  });

  it('Happy path: a transport failure leaves the client offline with no status (API-20)', async () => {
    const fake = createFakeTransport({ failure: new Error('connection refused') });

    const failure = await clientWith(fake)
      .getTemplate('fantasy_v1')
      .catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(DomainFailure);
    expect(failure).toMatchObject({ kind: 'offline' });
    expect((failure as DomainFailure).status).toBeUndefined();
    expect((failure as DomainFailure).code).toBeUndefined();
  });

  it('Edge: the offline failure carries no identifier-bearing text from the transport (API-29)', async () => {
    const fake = createFakeTransport({
      failure: new Error(`request to ${baseUrl}/sheets/secret-id failed`),
    });

    const failure = (await clientWith(fake)
      .getSheet('secret-id')
      .catch((error: unknown) => error)) as DomainFailure;

    expect(failure.message).not.toContain('secret-id');
    expect(failure.message).not.toContain(baseUrl);
  });

  it('Happy path: the classification table is total over the codes the backend emits (API-15, API-16, API-17, API-18, API-19, API-21, WCS-11)', () => {
    const codes = [
      'INVALID_JSON',
      'INVALID_REQUEST',
      'UNKNOWN_FIELD',
      'NOT_FOUND',
      'INTERNAL_ERROR',
      'CONFLICT',
      'CYCLE_DETECTED',
      'INVALID_DEPENDENCY',
      'INVALID_PATH',
      'SHEET_LIMIT_REACHED',
      'UNAUTHORIZED',
      'FORBIDDEN',
      'RATE_LIMITED',
      'OTP_INVALID',
      'EMAIL_UNAVAILABLE',
    ];

    for (const code of codes) {
      expect(apperrKind(code)).toMatch(
        /^(clientFault|invalidEdit|renderMismatch|notFound|serverError|unauthorized|forbidden|rateLimited|invalidCode|emailUnavailable|conflict|unknown)$/,
      );
    }
  });
});
