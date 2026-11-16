import { decodeEnvelope } from './envelope';
import { classifyDomainFailure, offlineFailure } from './error_mapper';
import { DefensiveParseFailure, DomainFailure } from './errors';
import type { HttpMethod, HttpRequest, HttpResponse, Transport } from './transport';

export const DEFAULT_REQUEST_TIMEOUT_MS = 5000;

export const DEFAULT_RETRY_DELAY_MS = 300;

export interface CallerOptions {
  readonly baseUrl: string;
  readonly transport: Transport;
  readonly timeoutMs?: number;
  readonly retryDelayMs?: number;
  readonly sleep?: (ms: number) => Promise<void>;
}

export interface CallOptions {
  readonly credentials?: 'omit' | 'include';
  readonly headers?: Readonly<Record<string, string>>;
}

export interface RequestCaller {
  call<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
    options?: CallOptions,
  ): Promise<T>;
  callNoContent(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    options?: CallOptions,
  ): Promise<void>;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isReceivedFailure(error: unknown): error is DomainFailure | DefensiveParseFailure {
  return error instanceof DomainFailure || error instanceof DefensiveParseFailure;
}

function classifyReceived(error: DomainFailure | DefensiveParseFailure): never {
  throw error instanceof DomainFailure ? classifyDomainFailure(error) : error;
}

export function createCaller(options: CallerOptions): RequestCaller {
  const { baseUrl, transport } = options;
  const timeoutMs = options.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS;
  const retryDelayMs = options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS;
  const sleep = options.sleep ?? defaultSleep;

  async function send(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    callOptions: CallOptions | undefined,
  ): Promise<HttpResponse> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);

    const request: HttpRequest = {
      url: `${baseUrl}${path}`,
      method,
      headers: {
        ...(body === undefined ? {} : { 'content-type': 'application/json' }),
        ...(callOptions?.headers ?? {}),
      },
      ...(body === undefined ? {} : { body }),
      credentials: callOptions?.credentials ?? 'omit',
      signal: controller.signal,
    };

    try {
      return await transport(request);
    } finally {
      clearTimeout(timer);
    }
  }

  async function call<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
    callOptions?: CallOptions,
  ): Promise<T> {
    const attempts = method === 'GET' ? 2 : 1;

    for (let index = 0; index < attempts; index += 1) {
      try {
        const response = await send(method, path, body, callOptions);
        return decodeEnvelope({
          status: response.status,
          body: response.body,
          decode,
          ...(response.retryAfter === undefined ? {} : { retryAfter: response.retryAfter }),
        });
      } catch (error) {
        if (isReceivedFailure(error)) {
          classifyReceived(error);
        }

        if (index < attempts - 1) {
          await sleep(retryDelayMs);
        }
      }
    }

    throw offlineFailure();
  }

  async function callNoContent(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    callOptions?: CallOptions,
  ): Promise<void> {
    let response: HttpResponse;

    try {
      response = await send(method, path, body, callOptions);
    } catch (error) {
      if (isReceivedFailure(error)) {
        classifyReceived(error);
      }
      throw offlineFailure();
    }

    if (response.status >= 200 && response.status < 300) {
      return;
    }

    try {
      decodeEnvelope({
        status: response.status,
        body: response.body,
        decode: () => undefined,
        ...(response.retryAfter === undefined ? {} : { retryAfter: response.retryAfter }),
      });
    } catch (error) {
      if (isReceivedFailure(error)) {
        classifyReceived(error);
      }
      throw offlineFailure();
    }

    throw offlineFailure();
  }

  return { call, callNoContent };
}
