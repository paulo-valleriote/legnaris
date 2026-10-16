/**
 * The typed operations of the consumed backend contract (API-01 … API-07).
 *
 * Every call goes through one place: the client builds the request, issues it
 * through the injected transport and decodes the envelope exactly once, so a
 * screen never writes a URL, unwraps a response or branches on a status.
 */

import { decodeEnvelope } from './envelope';
import { classifyDomainFailure, offlineFailure } from './error_mapper';
import { DefensiveParseFailure, DomainFailure } from './errors';
import type { Sheet } from './models/sheet';
import { parseSheet } from './models/sheet';
import type { TemplateDetail, TemplateSummary } from './models/template';
import { parseTemplateCatalog, parseTemplateDetail } from './models/template';
import { drift, isPlainObject } from './parse';
import type { HttpMethod, HttpRequest, Transport } from './transport';

/** The `/health` payload (API-02). */
export interface HealthPayload {
  readonly status: string;
}

/** The sheet-creation input (API-05). */
export interface CreateSheetInput {
  readonly templateId: string;
  readonly sheetName: string;
}

/** Construction options for {@link createApiClient} (API-07). */
export interface ApiClientOptions {
  /** Resolved absolute base address, no trailing slash (API-25, API-26). */
  readonly baseUrl: string;
  /** The single transport every request goes through (API-07). */
  readonly transport: Transport;
  /** Per-attempt fail-fast timeout; defaults to {@link DEFAULT_REQUEST_TIMEOUT_MS} (API-30). */
  readonly timeoutMs?: number;
  /** Fixed delay before the single read retry; defaults to {@link DEFAULT_RETRY_DELAY_MS} (API-31). */
  readonly retryDelayMs?: number;
  /** Injected delay for the read retry, so the interval is asserted without waiting (API-31). */
  readonly sleep?: (ms: number) => Promise<void>;
}

/** Fail-fast timeout bound on every request attempt, in milliseconds (API-30). */
export const DEFAULT_REQUEST_TIMEOUT_MS = 5000;

/** Fixed delay before the single read retry, in milliseconds (API-31). */
export const DEFAULT_RETRY_DELAY_MS = 300;

/** The consumed operations, one method each (API-01). */
export interface ApiClient {
  health(): Promise<HealthPayload>;
  listTemplates(): Promise<TemplateSummary[]>;
  getTemplate(id: string): Promise<TemplateDetail>;
  createSheet(input: CreateSheetInput): Promise<Sheet>;
  getSheet(id: string): Promise<Sheet>;
  patchSheet(id: string, changeSet: Record<string, unknown>): Promise<Record<string, unknown>>;
  previewSheet(id: string, changeSet: Record<string, unknown>): Promise<Record<string, unknown>>;
}

function decodeHealth(data: unknown): HealthPayload {
  if (!isPlainObject(data) || typeof data['status'] !== 'string') {
    throw drift('health payload is missing a string status');
  }

  return { status: data['status'] };
}

function decodeChangeSet(data: unknown): Record<string, unknown> {
  if (!isPlainObject(data)) {
    throw drift('sheet update payload is not an object');
  }

  return data;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isReceivedFailure(error: unknown): error is DomainFailure | DefensiveParseFailure {
  return error instanceof DomainFailure || error instanceof DefensiveParseFailure;
}

/**
 * Builds the client over a resolved base address and an injected transport
 * (API-07).
 *
 * Every attempt is bounded by a fail-fast timeout; a read that fails at the
 * transport is repeated exactly once after a fixed delay, while a response —
 * any status — ends the attempt, and a write never repeats (API-30 … API-33).
 */
export function createApiClient(options: ApiClientOptions): ApiClient {
  const { baseUrl, transport } = options;
  const timeoutMs = options.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS;
  const retryDelayMs = options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS;
  const sleep = options.sleep ?? defaultSleep;

  async function attempt<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
  ): Promise<T> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);

    const request: HttpRequest = {
      url: `${baseUrl}${path}`,
      method,
      headers: body === undefined ? {} : { 'content-type': 'application/json' },
      ...(body === undefined ? {} : { body }),
      credentials: 'omit',
      signal: controller.signal,
    };

    try {
      const response = await transport(request);

      return decodeEnvelope({ status: response.status, body: response.body, decode });
    } finally {
      clearTimeout(timer);
    }
  }

  async function call<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
  ): Promise<T> {
    const attempts = method === 'GET' ? 2 : 1;

    for (let index = 0; index < attempts; index += 1) {
      try {
        return await attempt(method, path, body, decode);
      } catch (error) {
        if (isReceivedFailure(error)) {
          throw error instanceof DomainFailure ? classifyDomainFailure(error) : error;
        }

        if (index < attempts - 1) {
          await sleep(retryDelayMs);
        }
      }
    }

    throw offlineFailure();
  }

  return {
    health: () => call('GET', '/health', undefined, decodeHealth),
    listTemplates: () => call('GET', '/templates', undefined, parseTemplateCatalog),
    getTemplate: (id) =>
      call('GET', `/templates/${encodeURIComponent(id)}`, undefined, parseTemplateDetail),
    createSheet: (input) =>
      call(
        'POST',
        '/sheets',
        JSON.stringify({ template_id: input.templateId, sheet_name: input.sheetName }),
        parseSheet,
      ),
    getSheet: (id) => call('GET', `/sheets/${encodeURIComponent(id)}`, undefined, parseSheet),
    patchSheet: (id, changeSet) =>
      call(
        'PATCH',
        `/sheets/${encodeURIComponent(id)}`,
        JSON.stringify({ data: changeSet }),
        decodeChangeSet,
      ),
    previewSheet: (id, changeSet) =>
      call(
        'POST',
        `/sheets/${encodeURIComponent(id)}/preview`,
        JSON.stringify({ data: changeSet }),
        decodeChangeSet,
      ),
  };
}
