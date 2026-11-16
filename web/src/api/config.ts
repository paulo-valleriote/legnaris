import { ConfigurationFailure } from './errors';

export interface ApiConfiguration {
  readonly baseUrl: string;
  readonly source: 'default' | 'override';
  readonly secure: boolean;
  readonly isDevOverride: boolean;
}

export const DEFAULT_API_BASE_URL = 'https://api-legnaris.paulovalleriote.io';

function parseBaseUrl(candidate: string): URL | undefined {
  try {
    return new URL(candidate);
  } catch {
    return undefined;
  }
}

export function resolveApiConfig(raw: string | undefined): ApiConfiguration {
  const override = raw?.trim() ?? '';
  const candidate = (override === '' ? DEFAULT_API_BASE_URL : override).replace(/\/+$/, '');
  const url = parseBaseUrl(candidate);

  if (url === undefined || (url.protocol !== 'http:' && url.protocol !== 'https:')) {
    throw new ConfigurationFailure(`base address "${candidate}" is not an absolute http(s) URL`);
  }

  return {
    baseUrl: candidate,
    source: override === '' ? 'default' : 'override',
    secure: url.protocol === 'https:',
    isDevOverride: override !== '',
  };
}

export function resolveApiConfigFromBuild(): ApiConfiguration {
  return resolveApiConfig(import.meta.env.VITE_API_BASE_URL);
}
