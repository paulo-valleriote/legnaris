import { describe, expect, it } from 'vitest';
import { apperrKind, failureMessage, generationErrorKind } from '../api/error_mapper';
import type { FailureKind } from '../api/errors';

const generationKinds: readonly FailureKind[] = [
  'generationInProgress',
  'planUpgradeRequired',
  'quotaExceeded',
  'llmConsentRequired',
  'providerUnavailable',
  'generationCapReached',
  'generationFailed',
  'generationTimeout',
  'sheetLimitReached',
];

describe('apperrKind for generation codes (LGN-196)', () => {
  const tests = [
    {
      name: 'Happy path: GENERATION_IN_PROGRESS',
      code: 'GENERATION_IN_PROGRESS',
      kind: 'generationInProgress',
    },
    {
      name: 'Happy path: PLAN_UPGRADE_REQUIRED',
      code: 'PLAN_UPGRADE_REQUIRED',
      kind: 'planUpgradeRequired',
    },
    { name: 'Happy path: QUOTA_EXCEEDED', code: 'QUOTA_EXCEEDED', kind: 'quotaExceeded' },
    {
      name: 'Happy path: LLM_CONSENT_REQUIRED',
      code: 'LLM_CONSENT_REQUIRED',
      kind: 'llmConsentRequired',
    },
    {
      name: 'Happy path: PROVIDER_UNAVAILABLE',
      code: 'PROVIDER_UNAVAILABLE',
      kind: 'providerUnavailable',
    },
    {
      name: 'Happy path: GENERATION_CAP_REACHED',
      code: 'GENERATION_CAP_REACHED',
      kind: 'generationCapReached',
    },
    {
      name: 'Edge: SHEET_LIMIT_REACHED is now mapped, not unknown',
      code: 'SHEET_LIMIT_REACHED',
      kind: 'sheetLimitReached',
    },
    {
      name: 'Edge: a lower-case generation code is not a backend code and stays unknown',
      code: 'quota_exceeded',
      kind: 'unknown',
    },
    { name: 'Edge: an empty code stays unknown', code: '', kind: 'unknown' },
  ] as const;

  for (const tc of tests) {
    it(tc.name, () => {
      expect(apperrKind(tc.code)).toBe(tc.kind);
    });
  }
});

describe('generationErrorKind (LGN-230 failure codes)', () => {
  const tests = [
    {
      name: 'Happy path: provider_unavailable is the provider outage kind',
      code: 'provider_unavailable',
      kind: 'providerUnavailable',
    },
    { name: 'Happy path: provider_failed', code: 'provider_failed', kind: 'generationFailed' },
    { name: 'Happy path: invalid_output', code: 'invalid_output', kind: 'generationFailed' },
    {
      name: 'Happy path: template_unavailable',
      code: 'template_unavailable',
      kind: 'generationFailed',
    },
    { name: 'Happy path: lease_expired', code: 'lease_expired', kind: 'generationFailed' },
    { name: 'Happy path: publish_failed', code: 'publish_failed', kind: 'generationFailed' },
    { name: 'Edge: an absent code is a plain failure', code: undefined, kind: 'generationFailed' },
    {
      name: 'Edge: an unknown code never names itself, it is a plain failure',
      code: 'something_new',
      kind: 'generationFailed',
    },
  ] as const;

  for (const tc of tests) {
    it(tc.name, () => {
      expect(generationErrorKind(tc.code)).toBe(tc.kind);
    });
  }
});

describe('failure copy is total over the generation kinds (LGN-196)', () => {
  for (const kind of generationKinds) {
    it(`Happy path: ${kind} resolves to non-empty copy from the catalog`, () => {
      const copy = failureMessage(kind);
      expect(copy.length).toBeGreaterThan(0);
      expect(copy).not.toMatch(/^api\.error\./);
    });
  }
});
