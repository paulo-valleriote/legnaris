import { type ApiConfiguration, resolveApiConfig } from './config';
import { ConfigurationFailure } from './errors';
import type { BrowserFetch } from './transport';

export const RUNTIME_CONFIG_PATH = '/config.json';

export interface LoadApiConfigurationOptions {
  readonly fetcher: BrowserFetch;
  readonly buildValue: string | undefined;
}

function failure(reason: string): ConfigurationFailure {
  return new ConfigurationFailure(`runtime configuration ${RUNTIME_CONFIG_PATH}: ${reason}`);
}

async function fetchConfigDocument(fetcher: BrowserFetch): Promise<unknown> {
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

export async function loadApiConfiguration(
  options: LoadApiConfigurationOptions,
): Promise<ApiConfiguration> {
  if ((options.buildValue ?? '').trim() !== '') {
    return resolveApiConfig(options.buildValue);
  }
  const document = await fetchConfigDocument(options.fetcher);
  return resolveApiConfig(readApiBaseUrl(document));
}
