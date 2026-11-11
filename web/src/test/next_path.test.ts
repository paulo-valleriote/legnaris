import { describe, expect, it } from 'vitest';
import { isSheetPath, sanitizeNextPath, signInPathFor } from '../auth/next_path';

describe('sanitizeNextPath', () => {
  const rejected: readonly (readonly [string, string | null | undefined])[] = [
    ['Edge: null', null],
    ['Edge: undefined', undefined],
    ['Edge: empty', ''],
    ['Edge: scheme-relative', '//evil.com'],
    ['Edge: absolute https', 'https://evil.com/sheets/x'],
    ['Edge: data scheme', 'data:text/html,x'],
    ['Edge: relative without slash', 'sheets/x'],
    ['Edge: slash backslash', '/\\evil.com'],
    ['Edge: backslash inside', '/sheets\\x'],
    ['Edge: encoded scheme-relative', '/%2Fevil.com'],
    ['Edge: encoded backslash', '/%5Cevil.com'],
    ['Edge: encoded newline', '/%0d%0a//evil.com'],
    ['Edge: tab smuggling', '/\t/evil.com'],
    ['Edge: malformed encoding', '/%E0%A4%A'],
    ['Edge: sign-in loop', '/sign-in?next=/sheets/x'],
    ['Edge: sign-up loop', '/sign-up'],
  ];

  it.each(rejected)('%s is rejected', (_name, value) => {
    expect(sanitizeNextPath(value)).toBeNull();
  });

  it.each(['/sheets/x', '/sheets/x?tab=1#roll', '/your-sheet', '/'])(
    'Happy path: %s is accepted verbatim',
    (value) => {
      expect(sanitizeNextPath(value)).toBe(value);
    },
  );
});

describe('signInPathFor', () => {
  it('Happy path: encodes the destination', () => {
    expect(signInPathFor('/sheets/x?a=1')).toBe('/sign-in?next=%2Fsheets%2Fx%3Fa%3D1');
  });

  it.each(['/', '//evil.com', '/sign-in'])('Edge: %s yields plain sign-in', (value) => {
    expect(signInPathFor(value)).toBe('/sign-in');
  });
});

describe('isSheetPath', () => {
  it('detects sheet destinations only', () => {
    expect(isSheetPath('/sheets/abc')).toBe(true);
    expect(isSheetPath('/sheets/')).toBe(false);
    expect(isSheetPath('/your-sheet')).toBe(false);
    expect(isSheetPath(null)).toBe(false);
  });
});
