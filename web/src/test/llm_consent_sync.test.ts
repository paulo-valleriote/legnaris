import { existsSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { CONSENT_VERSION, consentDisclosure } from '../generate/llm_consent';

const webCopyPath = new URL('../generate/llm_consent.json', import.meta.url);
const sourcePath = new URL('../../../docs/legal/llm-consent.json', import.meta.url);

function readJson(path: URL): unknown {
  return JSON.parse(readFileSync(path, 'utf8'));
}

describe('LLM consent disclosure (LGN-228)', () => {
  it('Happy path: the disclosure resolves the English locale with a version and an accept action', () => {
    const disclosure = consentDisclosure();

    expect(disclosure.title.length).toBeGreaterThan(0);
    expect(disclosure.points.length).toBeGreaterThan(0);
    expect(disclosure.acceptLabel.length).toBeGreaterThan(0);
    expect(disclosure.declineLabel.length).toBeGreaterThan(0);
  });

  it('Edge: the consent version is the disclosure version, a non-empty string', () => {
    expect(CONSENT_VERSION).toBe(consentDisclosure().version);
    expect(CONSENT_VERSION.length).toBeGreaterThan(0);
  });

  it('Edge: the web copy matches the source disclosure in docs/legal when the source is present', () => {
    if (!existsSync(sourcePath)) {
      return;
    }
    expect(readJson(webCopyPath)).toEqual(readJson(sourcePath));
  });
});
