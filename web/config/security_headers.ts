export const PRODUCTION_API_ORIGIN = 'https://api-legnaris.paulovalleriote.io';

export const DEV_API_ORIGIN = 'http://localhost:8080';

export function resolveDevApiOrigin(raw: string | undefined): string {
  try {
    const url = new URL((raw ?? '').trim());
    if (url.protocol === 'http:' || url.protocol === 'https:') {
      return url.origin;
    }
  } catch {}
  return DEV_API_ORIGIN;
}

export const REACT_REFRESH_PREAMBLE_HASH = "'sha256-Z2/iFzh9VMlVkEOar1f/oSHWwQk3ve1qk/C2WdsC4Xk='";

const ADSENSE_SCRIPT_ORIGINS = [
  'https://pagead2.googlesyndication.com',
  'https://fundingchoicesmessages.google.com',
  'https://*.adtrafficquality.google',
  'https://www.googletagservices.com',
].join(' ');

const ADSENSE_FRAME_ORIGINS = [
  'https://googleads.g.doubleclick.net',
  'https://tpc.googlesyndication.com',
  'https://*.googlesyndication.com',
  'https://fundingchoicesmessages.google.com',
  'https://www.google.com',
  'https://*.adtrafficquality.google',
].join(' ');

const ADSENSE_CONNECT_ORIGINS = [
  'https://pagead2.googlesyndication.com',
  'https://*.google.com',
  'https://*.adtrafficquality.google',
  'https://fundingchoicesmessages.google.com',
].join(' ');

export function buildSecurityHeaders(apiOrigin: string = PRODUCTION_API_ORIGIN, dev = false) {
  const scriptSrc = dev
    ? `script-src 'self' ${REACT_REFRESH_PREAMBLE_HASH} ${ADSENSE_SCRIPT_ORIGINS}`
    : `script-src 'self' ${ADSENSE_SCRIPT_ORIGINS}`;
  const styleSrc = dev ? "style-src 'self' 'unsafe-inline'" : "style-src 'self'";
  const contentSecurityPolicy = [
    "default-src 'self'",
    scriptSrc,
    styleSrc,
    "img-src 'self' data: https:",
    "font-src 'self'",
    `connect-src 'self' ${apiOrigin} ${ADSENSE_CONNECT_ORIGINS}`,
    `frame-src ${ADSENSE_FRAME_ORIGINS}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    "require-trusted-types-for 'script'",
    'report-to csp-endpoint',
    `report-uri ${apiOrigin}/csp-report`,
  ].join('; ');

  const headers: Record<string, string> = {
    'Content-Security-Policy': contentSecurityPolicy,
    'Reporting-Endpoints': `csp-endpoint="${apiOrigin}/csp-report"`,
    'X-Frame-Options': 'DENY',
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy':
      'camera=(), microphone=(), geolocation=(), payment=(), usb=(), fullscreen=(self)',
    'Cross-Origin-Opener-Policy': 'same-origin',
    'Cross-Origin-Resource-Policy': 'same-origin',
  };
  if (!dev) {
    headers['Strict-Transport-Security'] = 'max-age=63072000; includeSubDomains; preload';
  }
  return headers;
}
