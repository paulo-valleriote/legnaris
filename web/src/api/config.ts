/**
 * The API base address as configuration (API-25 … API-28).
 *
 * Resolved once from the build: the resolver is pure — raw value in, resolved
 * configuration out — so the whole policy is asserted without a browser, and the
 * build variable is read only at the module boundary.
 */

import { ConfigurationFailure } from './errors';

/** The resolved base address and where it came from (API-25 … API-28). */
export interface ApiConfiguration {
  /** Absolute `http(s)` address with no trailing slash. */
  readonly baseUrl: string;
  /** Whether the address came from the secure default or a build override. */
  readonly source: 'default' | 'override';
  /** True only for an `https:` address (API-28). */
  readonly secure: boolean;
  /** True when a build override supplied the value (API-26). */
  readonly isDevOverride: boolean;
}

/** The public API used when the build configures no base address (API-25). */
export const DEFAULT_API_BASE_URL = 'https://api-legnaris.paulovalleriote.io';

function parseBaseUrl(candidate: string): URL | undefined {
  try {
    return new URL(candidate);
  } catch {
    return undefined;
  }
}

/**
 * Resolves the raw build override into the configuration the client uses
 * (API-25 … API-28).
 *
 * An empty or whitespace override falls back to the HTTPS default; any value
 * that is not an absolute `http`/`https` address raises a configuration failure
 * here, at construction, before a request can be emitted.
 */
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

/**
 * Resolves the configuration from the build's environment (API-25, API-26).
 *
 * This is the only place the build variable is read; keeping it a thin boundary
 * leaves {@link resolveApiConfig} free of browser globals.
 */
export function resolveApiConfigFromBuild(): ApiConfiguration {
  return resolveApiConfig(import.meta.env.VITE_API_BASE_URL);
}
