import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const fixturesDir = path.resolve(here, '../api/fixtures');

const RULE =
  'web/AGENTS.md "API client" / "Standing decisions": captures are real responses, and a credential in one is redacted, never committed (API-35, LGN-295).';

const JWT = /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g;
const BEARER = /(authorization\s*:\s*)?bearer\s+([^\s"'`,;]+)/gi;
const REFRESH_JSON = /"(?:refresh_token|refreshToken)"\s*:\s*"([^"]*)"/g;
const COOKIE_PAIR = /([A-Za-z0-9_-]+)=([^;\s"'\\]+)/g;
const TOKEN_CANDIDATE = /[A-Za-z0-9+/=_-]{32,}/g;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isRedacted(value: string): boolean {
  return /^redacted(?:[-_][A-Za-z0-9-]+)*$/.test(value);
}

function jwtLike(text: string): string[] {
  return text.match(JWT) ?? [];
}

function bearerSecrets(text: string): string[] {
  const secrets: string[] = [];
  for (const match of text.matchAll(BEARER)) {
    const header = match[1] !== undefined;
    const token = match[2];
    if (token.startsWith('$') || isRedacted(token)) {
      continue;
    }
    if (!header && !(token.length >= 8 && /[0-9A-Z]/.test(token))) {
      continue;
    }
    secrets.push(token);
  }
  return secrets;
}

function unredactedRefreshValues(text: string): string[] {
  const values: string[] = [];

  for (const match of text.matchAll(REFRESH_JSON)) {
    if (match[1] !== '' && !isRedacted(match[1])) {
      values.push(match[1]);
    }
  }

  for (const line of text.split('\n')) {
    if (!/set-cookie/i.test(line)) {
      continue;
    }
    for (const match of line.matchAll(COOKIE_PAIR)) {
      if (/refresh|session|auth|token|jwt/i.test(match[1]) && !isRedacted(match[2])) {
        values.push(match[2]);
      }
    }
  }

  return values;
}

function looksHighEntropy(value: string): boolean {
  if (value.length < 32 || UUID.test(value)) {
    return false;
  }
  return [/[a-z]/, /[A-Z]/, /[0-9]/].every((pattern) => pattern.test(value));
}

function highEntropyTokens(text: string): string[] {
  return (text.match(TOKEN_CANDIDATE) ?? []).filter(looksHighEntropy);
}

function violationsIn(text: string): string[] {
  return [
    ...jwtLike(text).map((token) => `JWT-shaped string "${token}"`),
    ...bearerSecrets(text).map((token) => `Bearer credential "${token}"`),
    ...unredactedRefreshValues(text).map((value) => `un-redacted refresh value "${value}"`),
    ...highEntropyTokens(text).map((token) => `high-entropy token "${token}"`),
  ];
}

const fixtureFiles = readdirSync(fixturesDir, { withFileTypes: true })
  .filter((entry) => entry.isFile())
  .map((entry) => entry.name)
  .sort();

describe('the pinned captures carry no credential (LGN-295)', () => {
  it('Happy path: every file under src/api/fixtures is scanned and carries no token', () => {
    expect(fixtureFiles).toContain('capture.sh');
    expect(fixtureFiles).toContain('auth_challenge_verify.json');
    expect(fixtureFiles.length).toBeGreaterThan(10);

    const findings = fixtureFiles.flatMap((name) =>
      violationsIn(readFileSync(path.join(fixturesDir, name), 'utf8')).map(
        (violation) => `${name}: ${violation}`,
      ),
    );

    expect(findings, RULE).toEqual([]);
  });

  it('Happy path: the redacted placeholder the capture writes passes every rule', () => {
    expect(violationsIn('"access_token": "redacted-access-token"')).toEqual([]);
    expect(violationsIn('"refresh_token": "redacted-refresh-token"')).toEqual([]);
    expect(violationsIn('Authorization: Bearer redacted-access-token')).toEqual([]);
  });

  it('Error: a JWT-shaped string is reported', () => {
    const segment = (value: string) => btoa(value).replace(/=+$/, '');
    const jwt = [
      segment('{"alg":"HS256"}'),
      segment('{"sub":"1234567890"}'),
      segment('signature-value'),
    ].join('.');

    expect(jwtLike(jwt)).toEqual([jwt]);
    expect(violationsIn(jwt)).toHaveLength(1);
  });

  it('Edge: a shell variable or a redacted placeholder is not a Bearer credential', () => {
    expect(bearerSecrets('curl -H "Authorization: Bearer $TOKEN"')).toEqual([]);
    expect(bearerSecrets('Authorization: Bearer ${TOKEN}')).toEqual([]);
    expect(bearerSecrets('Authorization: Bearer redacted-access-token')).toEqual([]);
    expect(bearerSecrets('"message": "authorization bearer token is required"')).toEqual([]);
  });

  it('Error: a literal Bearer credential is reported', () => {
    const credential = 'abcdefgh'.repeat(2);

    expect(bearerSecrets(['Authorization:', 'Bearer', credential].join(' '))).toEqual([credential]);
  });

  it('Error: an un-redacted Set-Cookie refresh value is reported', () => {
    const value = 'abc123'.repeat(2);
    const header = `Set-Cookie: refresh_token=${value}; HttpOnly; Secure; Path=/auth`;

    expect(unredactedRefreshValues(header)).toEqual([value]);
    expect(
      unredactedRefreshValues('Set-Cookie: refresh_token=redacted-refresh-token; HttpOnly'),
    ).toEqual([]);
  });

  it('Error: an un-redacted refresh_token field is reported', () => {
    expect(unredactedRefreshValues('{"refresh_token": "a.b.c"}')).toEqual(['a.b.c']);
    expect(unredactedRefreshValues('{"refresh_token": "redacted-refresh-token"}')).toEqual([]);
    expect(unredactedRefreshValues('{"refresh_token": ""}')).toEqual([]);
  });

  it('Edge: only a mixed-class string of 32 or more characters reads as a high-entropy token', () => {
    expect(looksHighEntropy('aB3'.padEnd(32, 'x'))).toBe(true);
    expect(looksHighEntropy('01a14236-21c9-7546-945e-40c46d1fcd1d')).toBe(false);
    expect(looksHighEntropy('redacted-access-token')).toBe(false);
    expect(looksHighEntropy('alllowercaseanddigits1234567890')).toBe(false);
    expect(looksHighEntropy('aB3')).toBe(false);
  });

  it('Error: a mixed-class high-entropy token is reported', () => {
    const token = 'aB3'.padEnd(40, 'x');

    expect(highEntropyTokens(`value: ${token}`)).toEqual([token]);
  });
});
