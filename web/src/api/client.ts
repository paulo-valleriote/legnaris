/**
 * The six typed operations of the consumed backend contract (API-01 … API-07).
 *
 * Every call goes through one place: the client builds the request, issues it
 * through the injected transport and decodes the envelope exactly once, so a
 * screen never writes a URL, unwraps a response or branches on a status.
 */

import { decodeEnvelope } from './envelope';
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
}

/** The six consumed operations, one method each (API-01). */
export interface ApiClient {
  health(): Promise<HealthPayload>;
  listTemplates(): Promise<TemplateSummary[]>;
  getTemplate(id: string): Promise<TemplateDetail>;
  createSheet(input: CreateSheetInput): Promise<Sheet>;
  getSheet(id: string): Promise<Sheet>;
  patchSheet(id: string, changeSet: Record<string, unknown>): Promise<Record<string, unknown>>;
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

/**
 * Builds the client over a resolved base address and an injected transport
 * (API-07).
 *
 * The transport is captured once and used by every operation; no operation
 * creates its own, so credential attachment stays a single seam.
 */
export function createApiClient(options: ApiClientOptions): ApiClient {
  const { baseUrl, transport } = options;

  async function call<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
  ): Promise<T> {
    const request: HttpRequest = {
      url: `${baseUrl}${path}`,
      method,
      headers: body === undefined ? {} : { 'content-type': 'application/json' },
      ...(body === undefined ? {} : { body }),
      credentials: 'omit',
      signal: new AbortController().signal,
    };

    const response = await transport(request);

    return decodeEnvelope({ status: response.status, body: response.body, decode });
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
  };
}
