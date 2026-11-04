/**
 * The API base address resolved at runtime, so a single web image serves every
 * environment: the container writes `/config.json` from its environment at
 * start and the client reads it before the first render.
 */

import { type ApiConfiguration, resolveApiConfig } from './config';
import { ConfigurationFailure } from './errors';

/** Where the container publishes the runtime configuration. */
export const RUNTIME_CONFIG_PATH = '/config.json';

/** Dependencies of {@link loadApiConfiguration}, injectable for tests. */
export interface LoadApiConfigurationOptions {
  readonly fetcher: typeof fetch;
  /** The `VITE_API_BASE_URL` build value; when non-blank it wins and nothing is fetched. */
  readonly buildValue: string | undefined;
}

function failure(reason: string): ConfigurationFailure {
  return new ConfigurationFailure(`runtime configuration ${RUNTIME_CONFIG_PATH}: ${reason}`);
}

async function fetchConfigDocument(fetcher: typeof fetch): Promise<unknown> {
  let response: Response;
  try {
    response = await fetcher(RUNTIME_CONFIG_PATH, { cache: 'no-store' });
  } catch {
    throw failure('could not be loaded');
  }
  if (!response.ok) {
    throw failure(`responded with status ${response.status}`);
  }
  try {
    return (await response.json()) as unknown;
  } catch {
    throw failure('is not valid JSON');
  }
}

function readApiBaseUrl(document: unknown): string | undefined {
  if (typeof document !== 'object' || document === null || Array.isArray(document)) {
    throw failure('must be a JSON object');
  }
  const value = (document as { apiBaseUrl?: unknown }).apiBaseUrl;
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== 'string') {
    throw failure('apiBaseUrl must be a string');
  }
  return value;
}

/**
 * Resolves the API configuration: a non-blank build value (dev server, e2e)
 * wins; otherwise `/config.json` is fetched uncached. A blank or missing
 * `apiBaseUrl` falls back to the secure default; anything else invalid raises a
 * {@link ConfigurationFailure} before the app renders.
 */
export async function loadApiConfiguration(
  options: LoadApiConfigurationOptions,
): Promise<ApiConfiguration> {
  if ((options.buildValue ?? '').trim() !== '') {
    return resolveApiConfig(options.buildValue);
  }
  const document = await fetchConfigDocument(options.fetcher);
  return resolveApiConfig(readApiBaseUrl(document));
}
