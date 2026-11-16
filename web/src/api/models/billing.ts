import { drift, isPlainObject } from '../parse';

export const BILLING_PATHS = {
  checkout: '/billing/checkout',
  portal: '/billing/portal',
  refund: '/billing/refund',
} as const;

export function parseRedirectUrl(data: unknown): string {
  if (!isPlainObject(data)) {
    throw drift('billing answer is not an object');
  }

  const url = data['url'];
  if (typeof url !== 'string' || url === '') {
    throw drift('billing answer is missing a string url');
  }

  if (!isHttpsUrl(url)) {
    throw drift('billing answer url is not an https address');
  }

  return url;
}

function isHttpsUrl(candidate: string): boolean {
  try {
    return new URL(candidate).protocol === 'https:';
  } catch {
    return false;
  }
}
