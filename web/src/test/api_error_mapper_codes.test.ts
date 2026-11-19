import codesJson from '../../../testdata/apperr/codes.json';
import { describe, expect, it } from 'vitest';
import { apperrKind, failureMessage } from '../api/error_mapper';
import { messages } from '../l10n/strings';

interface CodesArtifact {
  readonly version: number;
  readonly codes: readonly { readonly code: string; readonly status: number }[];
}

const artifact = codesJson as CodesArtifact;

const RULE =
  'web/AGENTS.md "API client": a new backend code must be given a kind in error_mapper.ts; the mapping table is total over the codes the backend emits (LGN-284).';

describe('the shared apperr code artifact (LGN-284)', () => {
  it('Happy path: every declared backend code maps to a dedicated failure kind', () => {
    const missing = artifact.codes
      .filter((entry) => apperrKind(entry.code) === 'unknown')
      .map((entry) => entry.code);

    expect(missing, RULE).toEqual([]);
  });

  it('Happy path: every code in the artifact resolves a catalog message, never the generic copy', () => {
    for (const entry of artifact.codes) {
      expect(
        failureMessage(apperrKind(entry.code)),
        `"${entry.code}" must resolve its own catalog copy. ${RULE}`,
      ).not.toBe(messages['api.error.unknown']);
    }
  });

  it('Edge: the artifact is versioned, non-empty and each entry carries a code and an HTTP status', () => {
    expect(artifact.version).toBe(1);
    expect(artifact.codes.length).toBeGreaterThan(0);

    for (const entry of artifact.codes) {
      expect(entry.code).toMatch(/^[A-Z][A-Z0-9_]*$/);
      expect(entry.status).toBeGreaterThanOrEqual(400);
      expect(entry.status).toBeLessThan(600);
    }
  });

  it('Edge: the codes are unique', () => {
    const codes = artifact.codes.map((entry) => entry.code);

    expect(new Set(codes).size).toBe(codes.length);
  });
});
