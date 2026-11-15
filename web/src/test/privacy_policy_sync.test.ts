import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const LOCALES = ['privacy-policy.en.md', 'privacy-policy.pt-BR.md'] as const;

const HERE = dirname(fileURLToPath(import.meta.url));
const WEB_LEGAL_DIR = join(HERE, '..', 'legal');
const SOURCE_LEGAL_DIR = join(HERE, '..', '..', '..', 'docs', 'legal');

describe('privacy policy copies (LGN-234)', () => {
  it.each(LOCALES)('Happy path: the web copy of %s is byte-identical to docs/legal', (file) => {
    const webCopy = readFileSync(join(WEB_LEGAL_DIR, file));
    const source = readFileSync(join(SOURCE_LEGAL_DIR, file));

    expect(webCopy.equals(source)).toBe(true);
  });

  it.each(LOCALES)('Edge: the web copy of %s is not empty', (file) => {
    const webCopy = readFileSync(join(WEB_LEGAL_DIR, file));

    expect(webCopy.length).toBeGreaterThan(0);
  });
});
