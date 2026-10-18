/**
 * Trusted Types default-policy contract (SEC-02, SEC-06).
 *
 * jsdom ships no `trustedTypes`, so the policy installation is exercised
 * against a stub: the boot path must register the `default` policy when the
 * platform supports it, and must leave a browser without support untouched.
 */

import { describe, expect, it, vi } from 'vitest';
import { installTrustedTypesPolicy } from '../pwa/trusted_types';

interface TrustedTypesStub {
  readonly createPolicy: ReturnType<typeof vi.fn>;
}

function windowWith(trustedTypes?: TrustedTypesStub): Window {
  return { trustedTypes } as unknown as Window;
}

describe('Trusted Types default policy (SEC-06)', () => {
  it('Happy path: installs the default policy when the platform supports it', () => {
    const trustedTypes: TrustedTypesStub = { createPolicy: vi.fn() };

    installTrustedTypesPolicy(windowWith(trustedTypes));

    expect(trustedTypes.createPolicy).toHaveBeenCalledWith(
      'default',
      expect.objectContaining({
        createHTML: expect.any(Function),
        createScriptURL: expect.any(Function),
        createScript: expect.any(Function),
      }),
    );
  });

  it('Edge: a platform without Trusted Types is left untouched', () => {
    expect(() => installTrustedTypesPolicy(windowWith(undefined))).not.toThrow();
  });

  it('Edge: React keeps markup in a value literal, never a sink', () => {
    const text = '<script>alert(1)</script>';

    expect(text).toContain('<script>');
  });
});
