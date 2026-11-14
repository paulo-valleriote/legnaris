import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure } from '../api/errors';
import { parsePlanUsage } from '../api/models/plan';

const BASE = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
};

describe('plan consent fields (LGN-195)', () => {
  it('Happy path: a recorded consent decodes its date and version', () => {
    const usage = parsePlanUsage({
      ...BASE,
      llm_consent_at: '2026-11-01T12:30:00Z',
      llm_consent_version: '1',
    });

    expect(usage.consentAt).toBe('2026-11-01T12:30:00Z');
    expect(usage.consentVersion).toBe('1');
  });

  it('Edge: a payload without the consent keys reads as no consent recorded', () => {
    const usage = parsePlanUsage(BASE);

    expect(usage.consentAt).toBeNull();
    expect(usage.consentVersion).toBeNull();
  });

  it('Edge: explicit nulls (never consented or withdrawn) read as no consent recorded', () => {
    const usage = parsePlanUsage({
      ...BASE,
      llm_consent_at: null,
      llm_consent_version: null,
    });

    expect(usage.consentAt).toBeNull();
    expect(usage.consentVersion).toBeNull();
  });

  it('Edge: a version is kept as stored, even when it is not the current disclosure', () => {
    const usage = parsePlanUsage({
      ...BASE,
      llm_consent_at: '2026-10-01T00:00:00Z',
      llm_consent_version: '0',
    });

    expect(usage.consentVersion).toBe('0');
  });

  it.each([
    { name: 'a consent date that is not a timestamp', at: 'yesterday', version: '1' },
    { name: 'a consent date that is a number', at: 1_700_000_000, version: '1' },
    { name: 'a consent version that is a number', at: '2026-11-01T12:30:00Z', version: 1 },
  ])('Error: $name is a defensive parse failure', ({ at, version }) => {
    expect(() =>
      parsePlanUsage({ ...BASE, llm_consent_at: at, llm_consent_version: version }),
    ).toThrow(DefensiveParseFailure);
  });
});
