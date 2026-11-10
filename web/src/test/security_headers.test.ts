import { describe, expect, it } from 'vitest';
import {
  DEV_API_ORIGIN,
  PRODUCTION_API_ORIGIN,
  REACT_REFRESH_PREAMBLE_HASH,
  buildSecurityHeaders,
  resolveDevApiOrigin,
} from '../../config/security_headers';

describe('resolveDevApiOrigin', () => {
  it.each([
    ['Happy path: keeps a local origin', 'http://localhost:8080', 'http://localhost:8080'],
    ['Edge: strips a trailing slash', 'http://localhost:9000/', 'http://localhost:9000'],
    ['Edge: strips any path', 'http://localhost:9000/v1/api', 'http://localhost:9000'],
    ['Edge: trims surrounding whitespace', '  http://127.0.0.1:8080  ', 'http://127.0.0.1:8080'],
    ['Edge: undefined falls back', undefined, DEV_API_ORIGIN],
    ['Edge: empty falls back', '', DEV_API_ORIGIN],
    ['Edge: relative value falls back', '/api', DEV_API_ORIGIN],
    ['Error: non-http scheme falls back', 'data:text/plain,x', DEV_API_ORIGIN],
    ['Error: ws scheme falls back', 'ws://localhost:8080', DEV_API_ORIGIN],
  ])('%s', (_name, raw, expected) => {
    expect(resolveDevApiOrigin(raw)).toBe(expected);
  });
});

describe('buildSecurityHeaders', () => {
  it('Happy path: production names the production API and carries HSTS', () => {
    const headers = buildSecurityHeaders();

    expect(headers['Content-Security-Policy']).toContain(
      `connect-src 'self' ${PRODUCTION_API_ORIGIN};`,
    );
    expect(headers['Content-Security-Policy']).toContain(
      `report-uri ${PRODUCTION_API_ORIGIN}/csp-report`,
    );
    expect(headers['Reporting-Endpoints']).toBe(
      'csp-endpoint="https://api-legnaris.paulovalleriote.io/csp-report"',
    );
    expect(headers['Strict-Transport-Security']).toBe(
      'max-age=63072000; includeSubDomains; preload',
    );
  });

  it('Happy path: dev admits the local API and drops HSTS', () => {
    const headers = buildSecurityHeaders('http://localhost:8080', true);

    expect(headers['Content-Security-Policy']).toContain(
      "connect-src 'self' http://localhost:8080;",
    );
    expect(headers['Content-Security-Policy']).toContain(
      'report-uri http://localhost:8080/csp-report',
    );
    expect(headers['Reporting-Endpoints']).toBe('csp-endpoint="http://localhost:8080/csp-report"');
    expect(headers['Strict-Transport-Security']).toBeUndefined();
  });

  it('Edge: dev never leaves the production API in the policy', () => {
    const headers = buildSecurityHeaders('http://localhost:8080', true);

    expect(headers['Content-Security-Policy']).not.toContain('paulovalleriote.io');
  });

  it('Happy path: dev admits the React Refresh preamble by hash and inline styles, no inline script', () => {
    const csp = buildSecurityHeaders('http://localhost:8080', true)['Content-Security-Policy'];

    expect(csp).toContain(`script-src 'self' ${REACT_REFRESH_PREAMBLE_HASH};`);
    expect(csp).toContain("style-src 'self' 'unsafe-inline';");
    expect(csp).not.toMatch(/script-src[^;]*unsafe-inline/);
  });

  it('Edge: production never admits an inline script or style', () => {
    const csp = buildSecurityHeaders()['Content-Security-Policy'];

    expect(csp).toContain("script-src 'self';");
    expect(csp).toContain("style-src 'self';");
    expect(csp).not.toContain('sha256-');
    expect(csp).not.toContain('unsafe-inline');
  });

  it.each([true, false])('Edge: the rest of the policy is unchanged (dev=%s)', (dev) => {
    const csp = buildSecurityHeaders('http://localhost:8080', dev)['Content-Security-Policy'];

    expect(csp).toContain("default-src 'self'");
    expect(csp).toContain("frame-ancestors 'none'");
    expect(csp).toContain("require-trusted-types-for 'script'");
    expect(csp).toContain("object-src 'none'");
  });
});
