import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import { createFakeTransport, errorOutcome } from './fake_transport';

const baseUrl = 'http://api.test';

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

describe('revoke LLM consent (LGN-195, ADR 0008)', () => {
  it('Happy path: revokeLlmConsent issues one DELETE /me/llm-consent and accepts a 204', async () => {
    const fake = createFakeTransport({ status: 204, body: '' });

    await expect(clientWith(fake).revokeLlmConsent()).resolves.toBeUndefined();

    expect(fake.lastRequest?.method).toBe('DELETE');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/me/llm-consent`);
    expect(fake.lastRequest?.body).toBeUndefined();
    expect(fake.lastRequest?.credentials).toBe('omit');
    expect(fake.attempts).toBe(1);
  });

  it('Error: a failing revoke never repeats and surfaces a classified server failure', async () => {
    const fake = createFakeTransport(errorOutcome('INTERNAL_ERROR', 'boom', 500));

    const error = await clientWith(fake)
      .revokeLlmConsent()
      .catch((cause: unknown) => cause);

    expect(error).toBeInstanceOf(DomainFailure);
    expect(error).toMatchObject({ kind: 'serverError', status: 500 });
    expect(fake.attempts).toBe(1);
  });

  it('Error: an unauthorized revoke classifies as unauthorized and does not repeat', async () => {
    const fake = createFakeTransport(errorOutcome('UNAUTHORIZED', 'no session', 401));

    await expect(clientWith(fake).revokeLlmConsent()).rejects.toMatchObject({
      kind: 'unauthorized',
    });
    expect(fake.attempts).toBe(1);
  });
});
